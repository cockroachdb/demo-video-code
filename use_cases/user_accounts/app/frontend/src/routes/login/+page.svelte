<script>
  import { goto } from "$app/navigation";

  let email = "";
  let password = "";
  let error = "";

  async function handleSubmit(e) {
    await fetch("https://crdb-user-accounts.fly.dev/auth/login/", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email: email,
        password: password,
      }),
    })
      .then(async (data) => {
        return await data.json();
      })
      .then((j) => {
        goto(`/accounts/${j.id}/profile`).catch((error) => {
          console.error("Failed to navigate:", error);
        });
      })
      .catch((e) => {
        error = "invalid email or password";
      });
  }
</script>

<div class="min-h-screen bg-gray-100 flex items-center justify-center">
  <div
    class="max-w-md w-full bg-white p-8 border border-gray-300 rounded-lg shadow-md"
  >
    <h1 class="text-2xl font-bold text-center mb-4">Login</h1>

    <form on:submit={handleSubmit} class="space-y-6">
      <div>
        <input
          bind:value={email}
          type="email"
          placeholder="Email"
          required
          class="w-full p-4 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
        />
      </div>
      <div>
        <input
          bind:value={password}
          type="password"
          placeholder="Password"
          required
          class="w-full p-4 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
        />
      </div>
      <div>
        <input
          type="submit"
          value="Log in"
          class="w-full p-4 bg-brand-electric-purple text-white rounded-md cursor-pointer"
        />
      </div>

      {#if error}
        <p class="text-red-500">{error}</p>
      {/if}
    </form>
  </div>
</div>
