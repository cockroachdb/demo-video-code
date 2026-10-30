<script>
  export let data;
</script>

<div class="flex flex-col items-center justify-center min-h-screen bg-gray-100">
  <div class="w-full max-w-3xl px-4 py-8">
    <div class="overflow-hidden shadow-md sm:rounded-lg">
      <div class="px-4 py-5 bg-white space-y-6 sm:p-6">
        <div class="text-gray-800 text-sm font-mono whitespace-pre-wrap">{data.content}</div>
      </div>
    </div>
  </div>
</div>