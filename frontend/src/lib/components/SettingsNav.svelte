<script lang="ts">
  import { router } from "svelte-spa-router";

  const tabs = [
    { label: "Profile", href: "#/settings/profile", path: "/settings/profile" },
    {
      label: "Security",
      href: "#/settings/security",
      path: "/settings/security",
    },
    {
      label: "API Keys",
      href: "#/settings/api-keys",
      path: "/settings/api-keys",
    },
  ];
</script>

<div class="flex items-center gap-1 mb-6">
  {#each tabs as tab}
    <a
      href={tab.href}
      class="px-3 py-1.5 text-sm rounded-md transition-colors {router.location ===
      tab.path
        ? 'text-text bg-surface-muted font-medium'
        : 'text-muted hover:text-text hover:bg-surface-subtle'}"
    >
      {tab.label}
    </a>
  {/each}
</div>
