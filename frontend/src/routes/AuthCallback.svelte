<script lang="ts">
  import { onMount } from "svelte";
  import { push, querystring } from "svelte-spa-router";
  import { auth, toast } from "../lib/stores";

  onMount(() => {
    const params = new URLSearchParams($querystring);
    const token = params.get("token");
    const refresh = params.get("refresh");
    const error = params.get("error");

    if (error) {
      toast.error(decodeURIComponent(error));
      push("/login");
      return;
    }

    if (token && refresh) {
      auth.setTokens(token, refresh);
      toast.success("Logged in successfully");
      push("/");
    } else {
      toast.error("Invalid callback");
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
