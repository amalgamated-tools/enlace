<script lang="ts">
  export let type: 'text' | 'email' | 'password' | 'number' = 'text';
  export let value = '';
  export let placeholder = '';
  export let label = '';
  export let error = '';
  export let disabled = false;
  export let required = false;
  export let id = '';
  export let autocomplete: AutoFill | '' = '';

  const inputId = id || `input-${Math.random().toString(36).substring(2, 9)}`;
</script>

<div class="space-y-1.5">
  {#if label}
    <label for={inputId} class="block text-sm font-medium text-slate-700">
      {label}
      {#if required}<span class="text-red-500 ml-0.5">*</span>{/if}
    </label>
  {/if}
  <input
    id={inputId}
    {type}
    {placeholder}
    {disabled}
    {required}
    autocomplete={autocomplete || undefined}
    bind:value
    class="w-full px-3 py-2 text-sm bg-white border rounded-lg transition-colors duration-150 placeholder:text-slate-400 focus:outline-none focus:ring-2 focus:ring-slate-900/10 focus:border-slate-400 disabled:bg-slate-50 disabled:text-slate-400 {error ? 'border-red-400 focus:ring-red-500/10 focus:border-red-400' : 'border-slate-200'}"
    on:input
    on:blur
  />
  {#if error}
    <p class="text-xs text-red-500">{error}</p>
  {/if}
</div>
