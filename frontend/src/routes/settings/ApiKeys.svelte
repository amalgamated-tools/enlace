<script lang="ts">
  import { push } from "svelte-spa-router";
  import { ApiKeysSection, SettingsNav } from "../../lib/components";
  import { auth, isAuthenticated } from "../../lib/stores";

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }
</script>

<div>
  <h2 class="text-lg font-semibold text-text mb-6">Settings</h2>
  <SettingsNav />

  <ApiKeysSection />
</div>
