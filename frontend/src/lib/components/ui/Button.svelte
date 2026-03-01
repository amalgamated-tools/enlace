<script lang="ts">
  export let variant: "primary" | "secondary" | "danger" | "ghost" = "primary";
  export let size: "sm" | "md" | "lg" = "md";
  export let disabled = false;
  export let loading = false;
  export let type: "button" | "submit" = "button";

  const variants = {
    primary: "bg-slate-900 hover:bg-slate-800 text-white shadow-sm",
    secondary:
      "bg-white hover:bg-slate-50 text-slate-700 border border-slate-200 shadow-sm",
    danger: "bg-red-600 hover:bg-red-700 text-white shadow-sm",
    ghost: "hover:bg-slate-100 text-slate-600",
  };

  const sizes = {
    sm: "px-3 py-1.5 text-xs",
    md: "px-4 py-2 text-sm",
    lg: "px-5 py-2.5 text-base",
  };
</script>

<button
  {type}
  {disabled}
  class="inline-flex items-center justify-center rounded-lg font-medium transition-all duration-150 disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer {variants[
    variant
  ]} {sizes[size]}"
  on:click
>
  {#if loading}
    <svg
      class="animate-spin -ml-0.5 mr-2 h-3.5 w-3.5"
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
    >
      <circle
        class="opacity-25"
        cx="12"
        cy="12"
        r="10"
        stroke="currentColor"
        stroke-width="4"
      ></circle>
      <path
        class="opacity-75"
        fill="currentColor"
        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
      ></path>
    </svg>
  {/if}
  <slot />
</button>
