package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	screenWidth  = 800
	screenHeight = 800
	colorCount   = 10000
	angleSteps   = 100
	radiusSteps  = 100
)

func main() {
	db, err := pgxpool.New(context.Background(), "postgres://root@localhost:26257?sslmode=disable")
	if err != nil {
		log.Fatalf("error connecting to database: %v", err)
	}
	defer db.Close()

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Color Chooser")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	game := NewGame(db)

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}

type ColorPoint struct {
	color color.RGBA
	x, y  float32
}

type Game struct {
	db             *pgxpool.Pool
	colorPoints    []ColorPoint
	hoveredColor   *color.RGBA
	hoveredIndex   int
	centerX        float32
	centerY        float32
	maxRadius      float32
	lastLogTime    time.Time
	similarColors  []*color.RGBA
	lastFetchError error
}

func NewGame(db *pgxpool.Pool) *Game {
	g := &Game{
		db:            db,
		centerX:       screenWidth / 2,
		centerY:       screenHeight / 2,
		maxRadius:     screenHeight/2 - 20,
		hoveredIndex:  -1,
		lastLogTime:   time.Now(),
		similarColors: make([]*color.RGBA, 0),
	}
	g.colorPoints = g.generateCircularColors()
	return g
}

func hsvToRGB(h, s, v float64) color.RGBA {
	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := v - c

	var r, g, b float64

	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	return color.RGBA{
		R: uint8((r + m) * 255),
		G: uint8((g + m) * 255),
		B: uint8((b + m) * 255),
		A: 255,
	}
}

func (g *Game) generateCircularColors() []ColorPoint {
	points := make([]ColorPoint, 0, colorCount)

	for radiusStep := range radiusSteps {
		for angleStep := range angleSteps {
			angle := float64(angleStep) * 2 * math.Pi / float64(angleSteps)
			radiusFraction := float64(radiusStep) / float64(radiusSteps-1)
			radius := float32(radiusFraction) * g.maxRadius

			x := g.centerX + float32(math.Cos(angle))*radius
			y := g.centerY + float32(math.Sin(angle))*radius

			hue := float64(angleStep) * 360.0 / float64(angleSteps)
			saturation := radiusFraction
			value := 1.0

			c := hsvToRGB(hue, saturation, value)

			points = append(points, ColorPoint{
				color: c,
				x:     x,
				y:     y,
			})
		}
	}

	return points
}

func (g *Game) displaySimilarColours() {
	now := time.Now()

	// Enforce ~60fps.
	if now.Sub(g.lastLogTime) >= time.Second/60 {
		if g.hoveredColor != nil {
			colours, err := g.fetchSimilarColours(g.hoveredColor)
			if err != nil {
				log.Printf("Error fetching similar colours: %v", err)
				g.lastFetchError = err
				g.similarColors = make([]*color.RGBA, 0)
			} else {
				g.lastFetchError = nil
				g.similarColors = colours
			}
		} else {
			g.similarColors = make([]*color.RGBA, 0)
			g.lastFetchError = nil
		}
		g.lastLogTime = now
	}
}

func (g *Game) fetchSimilarColours(h *color.RGBA) ([]*color.RGBA, error) {
	const stmt = `SELECT rgb
								FROM colour
								ORDER BY rgb <-> $1
								LIMIT 5`

	vector := fmt.Sprintf("[%d,%d,%d]", h.R, h.G, h.B)

	rows, err := g.db.Query(context.Background(), stmt, vector)
	if err != nil {
		return nil, fmt.Errorf("fetching colours: %w", err)
	}
	defer rows.Close()

	var colours []*color.RGBA
	for rows.Next() {
		var rgbStr string
		if err = rows.Scan(&rgbStr); err != nil {
			return nil, fmt.Errorf("scanning colour: %w", err)
		}

		var r, g, b int
		if _, err = fmt.Sscanf(rgbStr, "[%d,%d,%d]", &r, &g, &b); err != nil {
			return nil, fmt.Errorf("parsing result into RGB values: %w", err)
		}

		colours = append(colours, &color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating rows: %w", err)
	}

	return colours, nil
}

func (g *Game) Update() error {
	mx, my := ebiten.CursorPosition()

	dx := float32(mx) - g.centerX
	dy := float32(my) - g.centerY
	distFromCenter := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if distFromCenter <= g.maxRadius {
		angle := math.Atan2(float64(dy), float64(dx))
		if angle < 0 {
			angle += 2 * math.Pi
		}

		angleStep := int(math.Round(angle / (2 * math.Pi) * float64(angleSteps)))
		if angleStep >= angleSteps {
			angleStep = 0
		}

		radiusStep := int(math.Round(float64(distFromCenter) / float64(g.maxRadius) * float64(radiusSteps-1)))
		if radiusStep >= radiusSteps {
			radiusStep = radiusSteps - 1
		}

		index := radiusStep*angleSteps + angleStep

		if index >= 0 && index < len(g.colorPoints) {
			g.hoveredIndex = index
			g.hoveredColor = &g.colorPoints[index].color
		} else {
			g.hoveredColor = nil
			g.hoveredIndex = -1
		}
	} else {
		g.hoveredColor = nil
		g.hoveredIndex = -1
	}

	g.displaySimilarColours()

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{30, 30, 30, 255})

	pointSize := float32(12.5)
	for _, cp := range g.colorPoints {
		vector.DrawFilledCircle(screen, cp.x, cp.y, pointSize, cp.color, false)
	}

	if g.hoveredColor != nil {
		vector.DrawFilledRect(screen, 10, 10, 300, 80, color.RGBA{0, 0, 0, 200}, false)

		info := fmt.Sprintf("RGB: (%d, %d, %d)\nHex: #%02X%02X%02X\nIndex: %d",
			g.hoveredColor.R, g.hoveredColor.G, g.hoveredColor.B,
			g.hoveredColor.R, g.hoveredColor.G, g.hoveredColor.B,
			g.hoveredIndex)

		ebitenutil.DebugPrintAt(screen, info, 20, 20)

		vector.DrawFilledRect(screen, 210, 20, 80, 60, *g.hoveredColor, false)
		vector.StrokeRect(screen, 210, 20, 80, 60, 2, color.White, false)
	}

	if len(g.similarColors) > 0 {
		vector.DrawFilledRect(screen, 10, 100, 300, 30, color.RGBA{0, 0, 0, 200}, false)
		ebitenutil.DebugPrintAt(screen, "Similar Colors from Database:", 20, 110)

		for i, simColor := range g.similarColors {
			yPos := float32(140 + i*70)

			vector.DrawFilledRect(screen, 10, yPos, 300, 60, color.RGBA{0, 0, 0, 200}, false)

			vector.DrawFilledRect(screen, 20, yPos+5, 50, 50, *simColor, false)
			vector.StrokeRect(screen, 20, yPos+5, 50, 50, 2, color.White, false)

			colorInfo := fmt.Sprintf("#%d: RGB(%d, %d, %d)\n     #%02X%02X%02X",
				i+1,
				simColor.R, simColor.G, simColor.B,
				simColor.R, simColor.G, simColor.B)
			ebitenutil.DebugPrintAt(screen, colorInfo, 80, int(yPos)+15)
		}
	}

	if g.lastFetchError != nil {
		vector.DrawFilledRect(screen, 10, 100, 300, 40, color.RGBA{100, 0, 0, 200}, false)
		ebitenutil.DebugPrintAt(screen, "Error fetching colors:", 20, 110)
		ebitenutil.DebugPrintAt(screen, "Check database connection", 20, 125)
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}
