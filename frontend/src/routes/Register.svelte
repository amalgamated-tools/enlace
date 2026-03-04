<script lang="ts">
  import { push } from "svelte-spa-router";
  import { Button, Input } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";

  let email = "";
  let password = "";
  let confirmPassword = "";
  let displayName = "";
  let loading = false;
  let errors: Record<string, string> = {};

  $: if ($isAuthenticated) {
    push("/");
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();
    errors = {};

    if (!email.trim()) {
      errors = { ...errors, email: "Email is required" };
    }
    if (!displayName.trim()) {
      errors = { ...errors, displayName: "Display name is required" };
    }
    if (!password) {
      errors = { ...errors, password: "Password is required" };
    } else if (password.length < 8) {
      errors = {
        ...errors,
        password: "Password must be at least 8 characters",
      };
    }
    if (password !== confirmPassword) {
      errors = { ...errors, confirmPassword: "Passwords do not match" };
    }

    if (Object.keys(errors).length > 0) {
      return;
    }

    loading = true;
    try {
      await auth.register(email, password, displayName);
      toast.success("Account created successfully");
      push("/");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Registration failed";
      toast.error(message);
    } finally {
      loading = false;
    }
  }
</script>

<div
  class="min-h-screen bg-surface-subtle flex items-center justify-center px-4"
>
  <div class="w-full max-w-sm">
    <div class="text-center mb-8">
      <h1 class="text-2xl font-semibold text-text">enlace</h1>
      <p class="text-sm text-muted mt-1">Create a new account</p>
    </div>

    <div class="bg-surface rounded-xl border border-border shadow-sm p-8">
      <form on:submit={handleSubmit} class="space-y-5">
        <Input
          type="text"
          label="Display Name"
          bind:value={displayName}
          placeholder="John Doe"
          error={errors.displayName}
          autocomplete="name"
          required
        />

        <Input
          type="email"
          label="Email"
          bind:value={email}
          placeholder="you@example.com"
          error={errors.email}
          autocomplete="email"
          required
        />

        <Input
          type="password"
          label="Password"
          bind:value={password}
          placeholder="At least 8 characters"
          error={errors.password}
          autocomplete="new-password"
          required
        />

        <Input
          type="password"
          label="Confirm Password"
          bind:value={confirmPassword}
          placeholder="Repeat your password"
          error={errors.confirmPassword}
          autocomplete="new-password"
          required
        />

        <Button type="submit" {loading} disabled={loading}>
          {loading ? "Creating account..." : "Create account"}
        </Button>
      </form>
    </div>

    <p class="mt-6 text-center text-sm text-muted">
      Already have an account?
      <a href="#/login" class="text-text font-medium hover:underline">Sign in</a
      >
    </p>
  </div>
</div>
