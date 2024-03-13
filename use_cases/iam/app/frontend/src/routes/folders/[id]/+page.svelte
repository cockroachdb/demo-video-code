<script>
  export let data;
</script>

<div class="flex flex-col items-center justify-center min-h-screen bg-gray-100">
  <div class="w-full max-w-3xl px-4 py-8">
    <div class="overflow-hidden shadow-md sm:rounded-lg">
      <div class="px-4 py-5 bg-white space-y-6 sm:p-6">
        <ul class="divide-y divide-gray-200">
          {#each data.files as {id, name, is_folder}}
            <li class="py-4 flex items-center justify-between">
              <span class="flex-grow text-sm font-medium text-gray-900">{name}</span>
              <a href={`/${is_folder ? 'folders' : 'files'}/${id}`} 
                 class="ml-4 flex-shrink-0">
                <button class="bg-blue-500 hover:bg-blue-700 text-white font-bold py-2 px-4 rounded">
                  {is_folder ? 'Open Folder' : 'View File'}
                </button>
              </a>
            </li>
          {/each}
        </ul>
      </div>
    </div>
  </div>
</div>