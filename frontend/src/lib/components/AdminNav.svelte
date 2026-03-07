<script lang="ts">
  import { location } from "svelte-spa-router";

  const tabs = [
    { label: "Users", href: "#/admin/users", path: "/admin/users" },
    { label: "Storage", href: "#/admin/storage", path: "/admin/storage" },
    { label: "Email", href: "#/admin/email", path: "/admin/email" },
    { label: "Webhooks", href: "#/admin/webhooks", path: "/admin/webhooks" },
  ];
</script>

<div class="flex items-center gap-1 mb-6">
  {#each tabs as tab}
    <a
      href={tab.href}
      class="px-3 py-1.5 text-sm rounded-md transition-colors {$location ===
      tab.path
        ? 'text-text bg-surface-muted font-medium'
        : 'text-muted hover:text-text hover:bg-surface-subtle'}"
    >
      {tab.label}
    </a>
  {/each}
</div>
