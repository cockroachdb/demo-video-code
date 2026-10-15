package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/samber/lo"
)

func main() {
	log.SetFlags(0)

	databaseURL, ok := os.LookupEnv("DATABASE_URL")
	if !ok {
		log.Fatalf("missing DATABASE_URL env var")
	}

	db, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}

	timeout, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err = db.Ping(timeout); err != nil {
		log.Fatalf("error testing database connection: %v", err)
	}

	region, err := loadRegion(db)
	if err != nil {
		log.Fatalf("error loading region: %v", err)
	}
	log.Printf("running in region: %q", region)

	loadProfileCache()
	go refreshProfiles(db)
	go refreshLeaseholders(db)
	go work(db, region)
	server(db)
}

func server(db *pgxpool.Pool) {
	router := fiber.New()
	router.Use(cors.New(cors.Config{ExposeHeaders: "X-Read-Latency-Ms, X-Offline-Regions"}))

	router.Get("/", handleGetAircraftDiagnostics(db))
	router.Get("/speed", handleGetSpeed(db))
	router.Get("/network", handleNetwork(db))
	router.Get("/cluster", handleCluster(db))
	router.Post("/nodes/:n/:action", handleNodeAction())
	router.Post("/regions/:r/:action", handleRegionAction())
	router.Post("/survival", handleSurvival(db))
	router.Get("/aircraft/:id/read", handleTimedRead(db))
	router.Post("/speed", handleSetSpeed(db))

	addr, ok := os.LookupEnv("LISTEN_ADDR")
	if !ok {
		addr = ":3000"
	}

	log.Fatal(router.Listen(addr))
}

// aircraftProfile is the slow-changing data from the GLOBAL aircraft and REGIONAL administration tables.
type aircraftProfile struct {
	Model, Base, Category string
	Capacity              int
	RangeKM, FlightHours  float64
	NextService           time.Time
}

// Cached profiles keep the map and workers running when those tables' home region is unreachable.
var (
	profiles   = map[string]aircraftProfile{}
	profilesMu sync.RWMutex
)

// profileCacheFile lets a restarted app draw the right aircraft even while those tables are unreachable.
const profileCacheFile = "/tmp/iot-profiles.json"

func loadProfileCache() {
	b, err := os.ReadFile(profileCacheFile)
	if err != nil {
		return
	}
	var cached map[string]aircraftProfile
	if json.Unmarshal(b, &cached) == nil && len(cached) > 0 {
		profilesMu.Lock()
		profiles = cached
		profilesMu.Unlock()
	}
}

func refreshProfiles(db *pgxpool.Pool) {
	const stmt = `SELECT a.id, a.model, a.base, a.category, a.capacity, a.range_km, adm.flight_hours, adm.next_service
								FROM aircraft a JOIN aircraft_administration adm ON adm.aircraft_id = a.id`
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		rows, err := db.Query(ctx, stmt)
		if err == nil {
			next := map[string]aircraftProfile{}
			for rows.Next() {
				var id string
				var p aircraftProfile
				if rows.Scan(&id, &p.Model, &p.Base, &p.Category, &p.Capacity, &p.RangeKM, &p.FlightHours, &p.NextService) == nil {
					next[id] = p
				}
			}
			if rows.Err() == nil && len(next) > 0 {
				profilesMu.Lock()
				profiles = next
				profilesMu.Unlock()
				if b, err := json.Marshal(next); err == nil {
					_ = os.WriteFile(profileCacheFile, b, 0o644)
				}
			}
			rows.Close()
		}
		cancel()
		time.Sleep(15 * time.Second)
	}
}

func profileOf(id string) aircraftProfile {
	profilesMu.RLock()
	defer profilesMu.RUnlock()
	return profiles[id]
}

// handleGetAircraftDiagnostics reads each region's telemetry in parallel, so one unreachable
// region only freezes that region's aircraft; it's reported in X-Offline-Regions.
func handleGetAircraftDiagnostics(db *pgxpool.Pool) fiber.Handler {
	const stmt = `SELECT aircraft_id, battery_level, st_x(location), st_y(location), heading,
									crdb_region, last_update, last_update_latency_ms
								FROM aircraft_diagnostics
								WHERE crdb_region = $1`

	return func(c *fiber.Ctx) error {
		start := time.Now()
		type result struct {
			region string
			rows   []aircraftDiagnostics
			err    error
		}
		results := make(chan result, len(warpRegions))
		for _, region := range warpRegions {
			go func(region string) {
				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				rows, err := db.Query(ctx, stmt, region)
				if err != nil {
					results <- result{region: region, err: err}
					return
				}
				defer rows.Close()
				var out []aircraftDiagnostics
				for rows.Next() {
					var d aircraftDiagnostics
					if err := rows.Scan(&d.AircraftID, &d.BatteryLevel, &d.Lon, &d.Lat, &d.Heading, &d.Region, &d.LastUpdate, &d.LastUpdateLatencyMS); err != nil {
						results <- result{region: region, err: err}
						return
					}
					p := profileOf(d.AircraftID)
					d.Model, d.Base, d.Category, d.Capacity = p.Model, p.Base, p.Category, p.Capacity
					d.RangeKM, d.FlightHours, d.NextService = p.RangeKM, p.FlightHours, p.NextService
					out = append(out, d)
				}
				results <- result{region: region, rows: out, err: rows.Err()}
			}(region)
		}

		diagnostics := []aircraftDiagnostics{}
		var offline []string
		for range warpRegions {
			r := <-results
			if r.err != nil {
				offline = append(offline, r.region)
				continue
			}
			diagnostics = append(diagnostics, r.rows...)
		}
		sort.Strings(offline)

		c.Set("X-Offline-Regions", strings.Join(offline, ","))
		c.Set("X-Read-Latency-Ms", fmt.Sprintf("%.1f", float64(time.Since(start).Microseconds())/1000))
		if len(offline) == len(warpRegions) {
			return fiber.NewError(fiber.StatusServiceUnavailable, "no region reachable")
		}
		return c.JSON(diagnostics)
	}
}

type aircraftDiagnostics struct {
	AircraftID          string    `json:"aircraft_id"`
	BatteryLevel        float64   `json:"battery_level"`
	Lat                 float64   `json:"lat"`
	Lon                 float64   `json:"lon"`
	Heading             int       `json:"heading"`
	Region              string    `json:"region"`
	LastUpdate          time.Time `json:"last_update"`
	LastUpdateLatencyMS int       `json:"last_update_latency_ms"`
	Model               string    `json:"model"`
	Base                string    `json:"base"`
	Category            string    `json:"category"`
	Capacity            int       `json:"capacity"`
	RangeKM             float64   `json:"range_km"`
	FlightHours         float64   `json:"flight_hours"`
	NextService         time.Time `json:"next_service"`
}

// handleTimedRead reads one aircraft by primary key across all three tables and reports how long it took.
func handleTimedRead(db *pgxpool.Pool) fiber.Handler {
	const stmt = `SELECT d.battery_level, a.model, adm.flight_hours
								FROM aircraft_diagnostics d
								JOIN aircraft a ON a.id = d.aircraft_id
								JOIN aircraft_administration adm ON adm.aircraft_id = d.aircraft_id
								WHERE d.aircraft_id = $1`

	return func(c *fiber.Ctx) error {
		timeout, cancel := context.WithTimeout(c.Context(), time.Second*2)
		defer cancel()

		var battery, hours float64
		var model string
		start := time.Now()
		if err := db.QueryRow(timeout, stmt, c.Params("id")).Scan(&battery, &model, &hours); err != nil {
			return fiber.NewError(fiber.StatusNotFound, "aircraft not found")
		}
		return c.JSON(fiber.Map{"read_latency_ms": float64(time.Since(start).Microseconds()) / 1000})
	}
}

type connectivityResponse struct {
	Connections map[string]struct {
		Peers map[string]struct {
			Latency  string `json:"latency"`
			Status   int    `json:"status"`
			Locality struct {
				Tiers []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"tiers"`
			} `json:"locality"`
		} `json:"peers"`
	} `json:"connections"`
}

// handleNetwork summarises CockroachDB's own measured node-to-node latency (from the
// /_status/connectivity endpoint) into per-region-pair round trips and live node counts.
// leaseTargets are the data whose leaseholder the map tracks: each region's telemetry partition,
// the GLOBAL fleet table and the EU-pinned admin table.
var leaseTargets = map[string]string{
	"region_1":       `SHOW RANGE FROM TABLE aircraft_diagnostics FOR ROW ('region_1', '00000000-0000-0000-0000-000000000000')`,
	"region_2":       `SHOW RANGE FROM TABLE aircraft_diagnostics FOR ROW ('region_2', '00000000-0000-0000-0000-000000000000')`,
	"region_3":       `SHOW RANGE FROM TABLE aircraft_diagnostics FOR ROW ('region_3', '00000000-0000-0000-0000-000000000000')`,
	"aircraft":       `SHOW RANGE FROM TABLE aircraft FOR ROW ('00000000-0000-0000-0000-000000000000')`,
	"administration": `SHOW RANGE FROM TABLE aircraft_administration FOR ROW ('00000000-0000-0000-0000-000000000000')`,
}

// nodeFromLocality maps a leaseholder locality like "region=region_2,zone=region_2c" to the
// container number (lat-roach6): three nodes per region, zones a/b/c.
func nodeFromLocality(locality string) (string, int) {
	var region, zone string
	for _, tier := range strings.Split(locality, ",") {
		if v, ok := strings.CutPrefix(tier, "region="); ok {
			region = v
		}
		if v, ok := strings.CutPrefix(tier, "zone="); ok {
			zone = v
		}
	}
	r, err := strconv.Atoi(strings.TrimPrefix(region, "region_"))
	if err != nil || zone == "" {
		return region, 0
	}
	return region, (r-1)*3 + int(zone[len(zone)-1]-'a') + 1
}

// Latest known leaseholder for each tracked piece of data. A lookup takes ~2s across the simulated WAN,
// so it's refreshed in the background and the last good answer is kept when one times out.
var (
	leaseCache   = map[string]fiber.Map{}
	leaseCacheMu sync.RWMutex
)

func refreshLeaseholders(db *pgxpool.Pool) {
	for {
		var wg sync.WaitGroup
		for key, show := range leaseTargets {
			wg.Add(1)
			go func(key, show string) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
				defer cancel()
				var locality string
				if err := db.QueryRow(ctx, "SELECT lease_holder_locality FROM ["+show+"]").Scan(&locality); err != nil {
					return
				}
				region, node := nodeFromLocality(locality)
				leaseCacheMu.Lock()
				leaseCache[key] = fiber.Map{"region": region, "node": node}
				leaseCacheMu.Unlock()
			}(key, show)
		}
		wg.Wait()
		time.Sleep(time.Second)
	}
}

func leaseholders() map[string]fiber.Map {
	leaseCacheMu.RLock()
	defer leaseCacheMu.RUnlock()
	out := make(map[string]fiber.Map, len(leaseCache))
	for k, v := range leaseCache {
		out[k] = v
	}
	return out
}

func handleNetwork(db *pgxpool.Pool) fiber.Handler {
	bases := strings.Split(os.Getenv("CRDB_HTTP"), ",")
	if bases[0] == "" {
		bases = []string{"http://localhost:8080", "http://localhost:8081", "http://localhost:8082"}
	}
	client := &http.Client{Timeout: 3 * time.Second}

	return func(c *fiber.Ctx) error {
		// Try each region's DB Console in turn so this keeps working when a whole region is down.
		var resp *http.Response
		var err error
		for _, base := range bases {
			if resp, err = client.Get(base + "/_status/connectivity"); err == nil && resp.StatusCode == http.StatusOK {
				break
			}
			if resp != nil {
				resp.Body.Close()
				resp = nil
			}
		}
		if resp == nil {
			return fiber.NewError(fiber.StatusBadGateway, "cluster status unavailable")
		}
		defer resp.Body.Close()

		var conn connectivityResponse
		if err := json.NewDecoder(resp.Body).Decode(&conn); err != nil {
			return fiber.NewError(fiber.StatusBadGateway, "bad cluster status")
		}

		nodeRegion := map[string]string{}
		nodeUp := map[string]bool{}
		for _, src := range conn.Connections {
			for id, peer := range src.Peers {
				for _, t := range peer.Locality.Tiers {
					if t.Key == "region" {
						nodeRegion[id] = t.Value
					}
				}
				if peer.Status == 1 {
					nodeUp[id] = true
				}
			}
		}
		for id := range conn.Connections {
			nodeUp[id] = true
		}

		samples := map[string][]float64{}
		for srcID, src := range conn.Connections {
			for id, peer := range src.Peers {
				a, b := nodeRegion[srcID], nodeRegion[id]
				d, err := time.ParseDuration(peer.Latency)
				if a == "" || b == "" || a == b || peer.Status != 1 || err != nil || d <= 0 {
					continue
				}
				if a > b {
					a, b = b, a
				}
				samples[a+"|"+b] = append(samples[a+"|"+b], float64(d.Microseconds())/1000)
			}
		}

		links := []fiber.Map{}
		for key, v := range samples {
			sort.Float64s(v)
			pair := strings.SplitN(key, "|", 2)
			links = append(links, fiber.Map{"a": pair[0], "b": pair[1], "rtt_ms": v[len(v)/2]})
		}

		nodes := map[string]fiber.Map{}
		for id, region := range nodeRegion {
			n := nodes[region]
			if n == nil {
				n = fiber.Map{"up": 0, "total": 0}
			}
			n["total"] = n["total"].(int) + 1
			if nodeUp[id] {
				n["up"] = n["up"].(int) + 1
			}
			nodes[region] = n
		}

		// Never report a lease on a node that's down: during failover the last known answer is stale.
		leases := leaseholders()
		if states, err := nodeStates(c.Context()); err == nil {
			for key, l := range leases {
				if n, _ := l["node"].(int); n > 0 && !states[n] {
					delete(leases, key)
				}
			}
		}
		served := map[string]string{}
		for _, region := range warpRegions {
			if l, ok := leases[region]; ok {
				served[region] = l["region"].(string)
			}
		}
		return c.JSON(fiber.Map{"links": links, "nodes": nodes, "served_by": served, "leases": leases})
	}
}

// Node controls for the local docker-compose.latency.yml cluster: lat-roach1..9, three per region.
const nodeCount = 9

func nodeContainer(n int) string { return fmt.Sprintf("lat-roach%d", n) }
func nodeRegionOf(n int) string  { return fmt.Sprintf("region_%d", (n-1)/3+1) }

func containerCLI() string {
	if v := os.Getenv("CONTAINER_CLI"); v != "" {
		return v
	}
	return "podman"
}

func runCLI(ctx context.Context, args ...string) error {
	out, err := exec.CommandContext(ctx, containerCLI(), args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %w: %s", containerCLI(), strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func nodeStates(ctx context.Context) (map[int]bool, error) {
	out, err := exec.CommandContext(ctx, containerCLI(), "ps", "-a", "--filter", "name=lat-roach", "--format", "{{.Names}} {{.State}}").Output()
	if err != nil {
		return nil, err
	}
	states := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || !strings.HasPrefix(f[0], "lat-roach") {
			continue
		}
		if n, err := strconv.Atoi(strings.TrimPrefix(f[0], "lat-roach")); err == nil {
			states[n] = f[1] == "running"
		}
	}
	return states, nil
}

var latencyMu sync.Mutex

// reapplyLatency re-adds the tc netem rules on every running node; a restarted node loses its own
// rules and may come back with a new IP that the other nodes' filters don't know about.
func reapplyLatency() {
	latencyMu.Lock()
	defer latencyMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	time.Sleep(3 * time.Second)

	states, err := nodeStates(ctx)
	if err != nil {
		log.Printf("error reading node states: %v", err)
		return
	}
	for n := 1; n <= nodeCount; n++ {
		if !states[n] {
			continue
		}
		if err := runCLI(ctx, "run", "--rm", "--net", "container:"+nodeContainer(n), "--cap-add", "NET_ADMIN",
			"-e", "MY_REGION="+nodeRegionOf(n), "localhost/lat-netem", "on"); err != nil {
			log.Printf("error re-applying latency on %s: %v", nodeContainer(n), err)
		}
	}
	log.Printf("re-applied simulated latency to running nodes")
}

func nodeAction(ctx context.Context, action string, nodes []int) error {
	args := []string{action}
	if action == "stop" {
		args = []string{"kill"}
	}
	for _, n := range nodes {
		args = append(args, nodeContainer(n))
	}
	if err := runCLI(ctx, args...); err != nil {
		return err
	}
	if action == "start" {
		go reapplyLatency()
	}
	return nil
}

func handleNodeAction() fiber.Handler {
	return func(c *fiber.Ctx) error {
		n, err := strconv.Atoi(c.Params("n"))
		action := c.Params("action")
		if err != nil || n < 1 || n > nodeCount || (action != "stop" && action != "start") {
			return fiber.NewError(fiber.StatusBadRequest, "expected /nodes/1-9/stop|start")
		}
		ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
		defer cancel()
		if err := nodeAction(ctx, action, []int{n}); err != nil {
			log.Printf("node action failed: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "node action failed")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleRegionAction() fiber.Handler {
	return func(c *fiber.Ctx) error {
		r, err := strconv.Atoi(strings.TrimPrefix(c.Params("r"), "region_"))
		action := c.Params("action")
		if err != nil || r < 1 || r > 3 || (action != "stop" && action != "start") {
			return fiber.NewError(fiber.StatusBadRequest, "expected /regions/region_1-3/stop|start")
		}
		ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
		defer cancel()
		states, err := nodeStates(ctx)
		if err != nil {
			return fiber.NewError(fiber.StatusBadGateway, "container status unavailable")
		}
		var nodes []int
		for n := (r-1)*3 + 1; n <= r*3; n++ {
			if states[n] == (action == "stop") {
				nodes = append(nodes, n)
			}
		}
		if len(nodes) == 0 {
			return c.SendStatus(fiber.StatusNoContent)
		}
		if err := nodeAction(ctx, action, nodes); err != nil {
			log.Printf("region action failed: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "region action failed")
		}
		return c.SendStatus(fiber.StatusNoContent)
	}
}

func handleCluster(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
		defer cancel()

		states, err := nodeStates(ctx)
		if err != nil {
			return fiber.NewError(fiber.StatusBadGateway, "container status unavailable")
		}
		nodes := []fiber.Map{}
		for n := 1; n <= nodeCount; n++ {
			nodes = append(nodes, fiber.Map{"n": n, "region": nodeRegionOf(n), "running": states[n]})
		}

		var survival string
		const stmt = `SELECT survival_goal FROM [SHOW DATABASES] WHERE database_name = 'iot'`
		if err := db.QueryRow(ctx, stmt).Scan(&survival); err != nil {
			survival = ""
		}
		return c.JSON(fiber.Map{"nodes": nodes, "survival": survival})
	}
}

func handleSurvival(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		goal := c.Query("goal")
		if goal != "zone" && goal != "region" {
			return fiber.NewError(fiber.StatusBadRequest, "goal must be zone or region")
		}
		ctx, cancel := context.WithTimeout(c.Context(), 60*time.Second)
		defer cancel()
		if _, err := db.Exec(ctx, fmt.Sprintf("ALTER DATABASE iot SURVIVE %s FAILURE", strings.ToUpper(goal))); err != nil {
			log.Printf("error setting survival goal: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error setting survival goal")
		}
		return c.JSON(fiber.Map{"survival": goal})
	}
}

func handleGetSpeed(db *pgxpool.Pool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"speed": loadSpeed(db)})
	}
}

func handleSetSpeed(db *pgxpool.Pool) fiber.Handler {
	allowed := map[float64]bool{1: true, 3: true, 5: true, 10: true}

	return func(c *fiber.Ctx) error {
		speed := c.QueryFloat("x")
		if !allowed[speed] {
			return fiber.NewError(fiber.StatusBadRequest, "speed must be 1, 3, 5 or 10")
		}

		const stmt = `UPSERT INTO demo_settings (key, value) VALUES ('speed', $1)`
		if _, err := db.Exec(c.Context(), stmt, speed); err != nil {
			log.Printf("error setting speed: %v", err)
			return fiber.NewError(fiber.StatusInternalServerError, "error setting speed")
		}

		return c.JSON(fiber.Map{"speed": speed})
	}
}

// Longitude bands inside each region, used to pick the UFO's next destination.
var warpTargets = map[string][2]float64{
	"region_1": {-170, -40},
	"region_2": {-15, 35},
	"region_3": {55, 170},
}

var warpRegions = []string{"region_1", "region_2", "region_3"}

// ufoTarget returns the point the UFO is flying toward, picking a new one in another
// region once it gets close. It's stored in demo_settings so whichever region's worker owns
// the UFO next keeps flying toward the same point.
func ufoTarget(db *pgxpool.Pool, region string, lat, lon float64) (float64, float64) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	target := map[string]float64{}
	rows, err := db.Query(ctx, `SELECT key, value FROM demo_settings WHERE key IN ('ufo_target_lat', 'ufo_target_lon')`)
	if err == nil {
		for rows.Next() {
			var k string
			var v float64
			if rows.Scan(&k, &v) == nil {
				target[k] = v
			}
		}
		rows.Close()
	}

	tLat, okLat := target["ufo_target_lat"]
	tLon, okLon := target["ufo_target_lon"]
	if okLat && okLon && distanceKM(lat, lon, tLat, tLon) > 800 {
		return tLat, tLon
	}

	band := warpTargets[lo.Sample(lo.Without(warpRegions, region))]
	tLat = -35 + rand.Float64()*85
	tLon = band[0] + rand.Float64()*(band[1]-band[0])
	const stmt = `UPSERT INTO demo_settings (key, value) VALUES ('ufo_target_lat', $1), ('ufo_target_lon', $2)`
	if _, err := db.Exec(ctx, stmt, tLat, tLon); err != nil {
		log.Printf("error saving ufo target: %v", err)
	}
	return tLat, tLon
}

func distanceKM(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dp, dl := p2-p1, (lon2-lon1)*math.Pi/180
	h := math.Sin(dp/2)*math.Sin(dp/2) + math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)
	return 2 * 6371 * math.Asin(math.Sqrt(h))
}

func bearingDeg(lat1, lon1, lat2, lon2 float64) float64 {
	p1, p2 := lat1*math.Pi/180, lat2*math.Pi/180
	dl := (lon2 - lon1) * math.Pi / 180
	y := math.Sin(dl) * math.Cos(p2)
	x := math.Cos(p1)*math.Sin(p2) - math.Sin(p1)*math.Cos(p2)*math.Cos(dl)
	return math.Mod(math.Atan2(y, x)*180/math.Pi+360, 360)
}

// loadSpeed reads the demo speed multiplier from the GLOBAL demo_settings table, defaulting to 1.
func loadSpeed(db *pgxpool.Pool) float64 {
	timeout, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	const stmt = `SELECT value FROM demo_settings WHERE key = 'speed'`
	var speed float64
	if err := db.QueryRow(timeout, stmt).Scan(&speed); err != nil {
		return lastSpeed
	}
	lastSpeed = speed
	return speed
}

// lastSpeed keeps the last known multiplier if demo_settings can't be read. Only the worker goroutine uses it.
var lastSpeed = 1.0

func work(db *pgxpool.Pool, region string) {
	for range time.NewTicker(time.Second * 1).C {
		multiplier := loadSpeed(db)

		aircraft, err := fetchAircraft(db, region)
		if err != nil {
			log.Printf("error fetching aircraft: %v", err)
			continue
		}

		if err = updateAircraft(db, aircraft, region, multiplier); err != nil {
			log.Printf("error moving aircraft: %v", err)
			continue
		}
	}
}

func fetchAircraft(db *pgxpool.Pool, region string) ([]aircraftDiagnostics, error) {
	const stmt = `SELECT aircraft_id, battery_level, ST_Y(location), ST_X(location), heading
								FROM aircraft_diagnostics
								WHERE crdb_region = $1`

	timeout, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	rows, err := db.Query(timeout, stmt, region)
	if err != nil {
		return nil, fmt.Errorf("querying aircraft: %w", err)
	}
	defer rows.Close()

	var results []aircraftDiagnostics
	for rows.Next() {
		r := aircraftDiagnostics{}
		if err = rows.Scan(&r.AircraftID, &r.BatteryLevel, &r.Lat, &r.Lon, &r.Heading); err != nil {
			return nil, fmt.Errorf("scanning aircraft: %w", err)
		}
		r.Category = profileOf(r.AircraftID).Category
		results = append(results, r)
	}

	return results, rows.Err()
}

// Last measured write round trip per aircraft, in ms.
var (
	lastWriteMS = map[string]int{}
	lastWriteMu sync.Mutex
)

// Kilometres travelled per one-second tick.
var speedKM = map[string]float64{
	"fixed_wing": 25,
	"helicopter": 15,
	"drone":      10,
	"cool_plane": 300,
}

func updateAircraft(db *pgxpool.Pool, aircraft []aircraftDiagnostics, region string, multiplier float64) error {
	const stmtFmt = `UPDATE aircraft_diagnostics SET
									   battery_level = $1,
										 location = 'POINT(%f %f)',
										 last_update = now(),
										 heading = $4,
										 last_update_latency_ms = COALESCE($3, last_update_latency_ms)
									 WHERE aircraft_id = $2`

	// Write every aircraft concurrently so cross-region write latency doesn't stretch the one-second tick.
	var wg sync.WaitGroup
	errs := make(chan error, len(aircraft))
	for _, a := range aircraft {
		heading := a.Heading
		speed, ok := speedKM[a.Category]
		if !ok {
			speed = 10
		}
		speed *= multiplier
		if a.Category == "ufo" {
			// Always fly toward a point in another region, wobbling, so the UFO keeps crossing boundaries.
			tLat, tLon := ufoTarget(db, region, a.Lat, a.Lon)
			heading = (int(bearingDeg(a.Lat, a.Lon, tLat, tLon)) + rand.Intn(71) - 35 + 360) % 360
			speed = 450 + rand.Float64()*250
		}

		lat, lon := moveAircraft(a.Lat, a.Lon, speed, float64(heading))

		// Turn back toward the equator so aircraft don't drift to the poles over a long-running demo.
		northbound := math.Cos(float64(heading)*math.Pi/180) > 0
		if (lat > 65 && northbound) || (lat < -50 && !northbound) {
			heading = (540 - heading) % 360
		}

		battery := a.BatteryLevel - 0.05
		if battery < 5 {
			battery = 100
		}

		// Record the previous write's measured round trip; timing on one clock avoids host/VM clock skew.
		lastWriteMu.Lock()
		var prevMS *int
		if ms, ok := lastWriteMS[a.AircraftID]; ok {
			prevMS = &ms
		}
		lastWriteMu.Unlock()

		wg.Add(1)
		go func(id string, stmt string, battery float64, heading int, prevMS *int) {
			defer wg.Done()
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			if _, err := db.Exec(ctx, stmt, battery, id, prevMS, heading); err != nil {
				errs <- fmt.Errorf("updating diagnostics: %w", err)
				return
			}
			lastWriteMu.Lock()
			lastWriteMS[id] = int(math.Round(float64(time.Since(start).Microseconds()) / 1000))
			lastWriteMu.Unlock()
		}(a.AircraftID, fmt.Sprintf(stmtFmt, lon, lat), battery, heading, prevMS)
	}
	wg.Wait()
	close(errs)

	return <-errs
}

func loadRegion(db *pgxpool.Pool) (string, error) {
	// Allow the environment to override the region.
	region, ok := os.LookupEnv("REGION")
	if ok {
		return region, nil
	}

	const stmt = `SELECT gateway_region()`

	row := db.QueryRow(context.Background(), stmt)
	if err := row.Scan(&region); err != nil {
		return "", fmt.Errorf("fetching gateway region: %w", err)
	}

	return region, nil
}

func moveAircraft(lat, lon, distance, bearing float64) (float64, float64) {
	dr := distance / 6371

	bearing = (bearing * (math.Pi / 180.0))

	lat1 := (lat * (math.Pi / 180.0))
	lng1 := (lon * (math.Pi / 180.0))

	lat2_part1 := math.Sin(lat1) * math.Cos(dr)
	lat2_part2 := math.Cos(lat1) * math.Sin(dr) * math.Cos(bearing)

	lat2 := math.Asin(lat2_part1 + lat2_part2)

	lng2_part1 := math.Sin(bearing) * math.Sin(dr) * math.Cos(lat1)
	lng2_part2 := math.Cos(dr) - (math.Sin(lat1) * math.Sin(lat2))

	lng2 := lng1 + math.Atan2(lng2_part1, lng2_part2)
	lng2 = math.Mod((lng2+3*math.Pi), (2*math.Pi)) - math.Pi

	lat2 = lat2 * (180.0 / math.Pi)
	lng2 = lng2 * (180.0 / math.Pi)

	return lat2, lng2
}
