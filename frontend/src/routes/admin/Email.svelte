<script lang="ts">
  import { onMount } from "svelte";
  import { push } from "svelte-spa-router";
  import { Button, Input, Modal, AdminNav } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import { api, ApiError } from "../../lib/api";

  interface SMTPConfig {
    smtp_host: string;
    smtp_port?: string;
    smtp_user?: string;
    smtp_pass_set: boolean;
    smtp_from?: string;
    smtp_tls_policy?: string;
  }

  let loading = true;
  let saving = false;
  let errors: Record<string, string> = {};

  // Form state — empty string means no DB override is set
  let smtpHost = "";
  let smtpPort = "";
  let smtpUser = "";
  let smtpPass = "";
  let smtpPassSet = false;
  let smtpFrom = "";
  let smtpTlsPolicy = "";

  // Reset modal
  let resetModal = false;
  let resetting = false;

  // Password clearing
  let clearPassword = false;

  $: hasOverrides =
    smtpHost !== "" ||
    smtpPort !== "" ||
    smtpUser !== "" ||
    smtpPassSet ||
    smtpFrom !== "" ||
    smtpTlsPolicy !== "";

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.initialized && $isAuthenticated && !$isAdmin) {
    toast.error("Access denied");
    push("/");
  }

  onMount(async () => {
    await loadConfig();
  });

  function applyConfig(config: SMTPConfig) {
    smtpHost = config.smtp_host || "";
    smtpPort = config.smtp_port || "";
    smtpUser = config.smtp_user || "";
    smtpPassSet = config.smtp_pass_set;
    smtpPass = "";
    clearPassword = false;
    smtpFrom = config.smtp_from || "";
    smtpTlsPolicy = config.smtp_tls_policy || "";
  }

  async function loadConfig() {
    if (!$isAdmin) return;

    loading = true;
    try {
      const config = await api.get<SMTPConfig>("/admin/smtp");
      applyConfig(config);
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to load SMTP configuration";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  async function handleSave(e: Event) {
    e.preventDefault();
    errors = {};

    if (!smtpHost.trim()) {
      errors = { smtp_host: "SMTP host is required to configure email" };
      return;
    }

    const payload: Record<string, string> = {
      smtp_host: smtpHost.trim(),
      smtp_from: smtpFrom.trim(),
    };

    if (smtpPort.trim()) {
      payload.smtp_port = smtpPort.trim();
    }
    if (smtpUser.trim()) {
      payload.smtp_user = smtpUser.trim();
    }
    if (smtpPass.trim()) {
      payload.smtp_pass = smtpPass.trim();
    } else if (clearPassword) {
      payload.smtp_pass = "";
    }
    if (smtpTlsPolicy) {
      payload.smtp_tls_policy = smtpTlsPolicy;
    }

    if (!smtpFrom.trim()) {
      errors = { ...errors, smtp_from: "From address is required" };
    }

    if (Object.keys(errors).length > 0) {
      return;
    }

    saving = true;
    try {
      const config = await api.put<SMTPConfig>("/admin/smtp", payload);
      applyConfig(config);
      toast.success("SMTP configuration saved");
    } catch (err) {
      if (err instanceof ApiError && err.fields) {
        errors = err.fields;
      } else {
        const message =
          err instanceof Error
            ? err.message
            : "Failed to save SMTP configuration";
        toast.error(message);
      }
    } finally {
      saving = false;
    }
  }

  async function handleReset() {
    resetting = true;
    try {
      await api.delete<void>("/admin/smtp");
      smtpHost = "";
      smtpPort = "";
      smtpUser = "";
      smtpPass = "";
      smtpPassSet = false;
      clearPassword = false;
      smtpFrom = "";
      smtpTlsPolicy = "";
      resetModal = false;
      toast.success(
        "SMTP configuration reset to environment variable defaults",
      );
    } catch (err) {
      const message =
        err instanceof Error
          ? err.message
          : "Failed to reset SMTP configuration";
      toast.error(message);
    } finally {
      resetting = false;
    }
  }
</script>

<AdminNav />

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else}
  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">SMTP Configuration</h3>
    </div>
    <div class="p-6">
      {#if !hasOverrides}
        <div class="mb-5 rounded-lg bg-surface-subtle px-4 py-3">
          <p class="text-sm text-muted">
            No database overrides are configured. Email is using environment
            variable defaults.
          </p>
        </div>
      {/if}

      <form on:submit={handleSave} class="space-y-5">
        <Input
          label="SMTP Host"
          bind:value={smtpHost}
          placeholder="smtp.example.com"
          error={errors.smtp_host}
        />

        <div class="grid gap-5 sm:grid-cols-2">
          <Input
            label="Port"
            bind:value={smtpPort}
            placeholder="587"
            error={errors.smtp_port}
          />
          <Input
            label="From Address"
            bind:value={smtpFrom}
            placeholder="noreply@example.com"
            error={errors.smtp_from}
          />
        </div>

        <fieldset>
          <legend class="block text-sm font-medium text-text mb-2"
            >TLS Policy</legend
          >
          {#if errors.smtp_tls_policy}
            <p class="text-xs text-danger mb-2">{errors.smtp_tls_policy}</p>
          {/if}
          <div class="flex items-center gap-6">
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                bind:group={smtpTlsPolicy}
                value="opportunistic"
                class="w-4 h-4 text-accent border-border focus:ring-accent/20"
              />
              <span class="text-sm text-text">Opportunistic</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                bind:group={smtpTlsPolicy}
                value="mandatory"
                class="w-4 h-4 text-accent border-border focus:ring-accent/20"
              />
              <span class="text-sm text-text">Mandatory</span>
            </label>
            <label class="flex items-center gap-2 cursor-pointer">
              <input
                type="radio"
                bind:group={smtpTlsPolicy}
                value="none"
                class="w-4 h-4 text-accent border-border focus:ring-accent/20"
              />
              <span class="text-sm text-text">None</span>
            </label>
          </div>
        </fieldset>

        <div class="grid gap-5 sm:grid-cols-2">
          <Input
            label="Username"
            bind:value={smtpUser}
            placeholder="(optional)"
            error={errors.smtp_user}
            autocomplete="off"
          />
          <Input
            type="password"
            label="Password"
            bind:value={smtpPass}
            placeholder={smtpPassSet
              ? "Leave blank to keep current"
              : "(optional)"}
            error={errors.smtp_pass}
            autocomplete="off"
            disabled={clearPassword}
          />
        </div>
        {#if smtpPassSet && !smtpPass.trim()}
          <label class="flex items-center gap-2 cursor-pointer -mt-3">
            <input
              type="checkbox"
              bind:checked={clearPassword}
              class="w-4 h-4 text-accent border-border focus:ring-accent/20 rounded"
            />
            <span class="text-sm text-muted">Clear saved password</span>
          </label>
        {/if}

        {#if smtpHost}
          <Button type="submit" loading={saving}>
            {saving ? "Saving..." : "Save Configuration"}
          </Button>
        {/if}
      </form>
    </div>
  </div>

  <div class="bg-surface rounded-xl border border-border mb-6">
    <div class="px-6 py-4 border-b border-border">
      <h3 class="text-sm font-semibold text-text">Reset to Defaults</h3>
    </div>
    <div class="p-6">
      <p class="text-sm text-muted mb-4">
        Remove all database overrides and revert to environment variable
        configuration on the next restart.
      </p>
      <Button variant="danger" on:click={() => (resetModal = true)}>
        Reset to Defaults
      </Button>
    </div>
  </div>

  <p class="text-xs text-subtle">
    Changes take effect after application restart.
  </p>
{/if}

<Modal
  open={resetModal}
  title="Reset SMTP Configuration"
  on:close={() => (resetModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to reset SMTP configuration? All database overrides
    will be removed and the application will use environment variable defaults
    on the next restart.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (resetModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={resetting} on:click={handleReset}
      >Reset</Button
    >
  </div>
</Modal>
