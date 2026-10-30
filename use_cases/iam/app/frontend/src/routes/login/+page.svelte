<script>
  import { enhance } from '$app/forms';
  
  export let form;
</script>

<div class="min-h-screen bg-gray-100 flex items-center justify-center">
  <div
    class="max-w-md w-full bg-white p-8 border border-gray-300 rounded-lg shadow-md"
  >
    <h1 class="text-2xl font-bold text-center mb-4">Login</h1>

    <form class="space-y-6" method="POST" use:enhance>
      <div>
        <input
          name="username"
          type="text"
          placeholder="Username"
          required
          class="w-full p-4 border border-gray-300 rounded-md focus:ring-blue-500 focus:border-blue-500"
        />
      </div>
      <div>
        <input
          type="submit"
          value="Login"
          class="w-full p-4 bg-brand-electric-purple text-white rounded-md cursor-pointer"
        />
      </div>

      {#if form}
        <p class="text-red-500">{form.error}</p>
      {/if}
    </form>
  </div>
</div>
