<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { auth, toast } from "../lib/stores";

  onMount(async () => {
    try {
      const response = await fetch("/api/v1/auth/oidc/exchange", {
        method: "POST",
      });
      const data = await response.json();

      if (!response.ok || !data.success) {
        toast.error(data.error || "Authentication failed");
        push("/login");
        return;
      }

      auth.setTokens(data.data.access_token, data.data.refresh_token);
      toast.success("Logged in successfully");
      push("/");
    } catch (err) {
      const message = err instanceof Error ? err.message : "Authentication failed";
      toast.error(message);
      push("/login");
    }
  });
</script>

<div class="min-h-screen bg-surface-subtle flex items-center justify-center">
  <div class="text-center">
    <div
      class="animate-spin rounded-full h-8 w-8 border-b-2 border-accent mx-auto"
    ></div>
    <p class="mt-4 text-muted">Completing login...</p>
  </div>
</div>
