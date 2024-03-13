<script>
  import "../app.css";
</script>

<div class="min-h-screen bg-gray-100">
  <nav class="bg-white shadow">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex justify-between h-16">
        <div class="flex">
        </div>
        <div class="flex items-center">
          <a href="/login" class="text-sm font-medium text-gray-500 hover:text-gray-900 mr-4">
            Login
          </a>
          <a href="/logout" class="text-sm font-medium text-gray-500 hover:text-gray-900">
            Logout
          </a>
        </div>
      </div>
    </div>
  </nav>
  <slot></slot>
</div>