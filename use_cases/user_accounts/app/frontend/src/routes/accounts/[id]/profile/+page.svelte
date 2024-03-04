<script>
  export let data;
</script>

<div class="min-h-screen bg-gray-100 flex flex-col items-center justify-center">
  <h1 class="text-2xl font-bold text-center mb-4">Profile</h1>
  <div class="bg-white p-6 rounded-lg shadow-lg text-center">
    <div class="mb-4">
      <img src="data:image/png;base64, {data.image}" alt="User Avatar" class="w-32 h-32 rounded-full mx-auto border-4 border-brand-electric-purple"/>
    </div>
    <p class="text-gray-600">{data.email}</p>
    <div class="mt-4">
      <p class="text-sm text-gray-500">Member since: {new Date(data.registered_on).toLocaleDateString()}</p>
    </div>
  </div>
</div>