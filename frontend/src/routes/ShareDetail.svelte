<script lang="ts">
  import { onMount } from "svelte";
  import { get } from "svelte/store";
  import { push } from "svelte-spa-router";
  import {
    Button,
    Input,
    FileUploader,
    FileList,
    Modal,
  } from "../lib/components";
  import { auth, isAuthenticated, toast, emailConfigured } from "../lib/stores";
  import {
    sharesApi,
    filesApi,
    dateToRFC3339,
    type Share,
    type FileInfo,
    type ShareRecipient,
  } from "../lib/api";

  export let params: { id: string } = { id: "" };

  let share: Share | null = null;
  let files: FileInfo[] = [];
  let recipients: ShareRecipient[] = [];
  let loading = true;
  let saving = false;
  let uploading = false;

  let editMode = false;
  let editName = "";
  let editDescription = "";
  let editPassword = "";
  let editMaxDownloads = "";
  let editMaxViews = "";
  let editExpiresAt = "";

  let deleteModal = false;
  let deleting = false;

  let notifyModal = false;
  let notifyEmails = "";
  let notifying = false;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: shareUrl = share ? `${window.location.origin}/#/s/${share.slug}` : "";

  onMount(async () => {
    await loadShare();
  });

  async function loadShare() {
    if (!$isAuthenticated || !params.id) return;

    loading = true;
    try {
      const shareData = await sharesApi.get(params.id);
      share = shareData;

      editName = shareData.name;
      editDescription = shareData.description || "";
      editMaxDownloads = shareData.max_downloads
        ? String(shareData.max_downloads)
        : "";
      editMaxViews = shareData.max_views ? String(shareData.max_views) : "";
      editExpiresAt = shareData.expires_at
        ? shareData.expires_at.split("T")[0]
        : "";

      const response = await fetch(`/api/v1/shares/${params.id}/files`, {
        headers: {
          Authorization: `Bearer ${localStorage.getItem("access_token")}`,
        },
      });
      const data = await response.json();
      if (data.success) {
        files = data.data || [];
      }

      try {
        const recipientData = await sharesApi.getRecipients(params.id);
        recipients = recipientData || [];
      } catch {
        recipients = [];
      }
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load share";
      toast.error(message);
      push("/shares");
    } finally {
      loading = false;
    }
  }

  async function handleSave() {
    if (!share) return;

    saving = true;
    try {
      const updated = await sharesApi.update(share.id, {
        name: editName,
        description: editDescription || undefined,
        password: editPassword || undefined,
        max_downloads: editMaxDownloads
          ? parseInt(editMaxDownloads, 10)
          : undefined,
        max_views: editMaxViews ? parseInt(editMaxViews, 10) : undefined,
        expires_at: editExpiresAt ? dateToRFC3339(editExpiresAt) : undefined,
        clear_expiry: share.expires_at && !editExpiresAt ? true : undefined,
      });
      share = updated;
      editPassword = "";
      editMode = false;
      toast.success("Share updated");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to update share";
      toast.error(message);
    } finally {
      saving = false;
    }
  }

  async function handleFileUpload(event: CustomEvent<File[]>) {
    if (!share) return;

    uploading = true;
    try {
      const newFiles = await filesApi.upload(share.id, event.detail);
      files = [...files, ...newFiles];
      toast.success(`${event.detail.length} file(s) uploaded`);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Upload failed";
      toast.error(message);
    } finally {
      uploading = false;
    }
  }

  async function handleFileDelete(event: CustomEvent<string>) {
    try {
      await filesApi.delete(event.detail);
      files = files.filter((f) => f.id !== event.detail);
      toast.success("File deleted");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete file";
      toast.error(message);
    }
  }

  async function handleDeleteShare() {
    if (!share) return;

    deleting = true;
    try {
      await sharesApi.delete(share.id);
      toast.success("Share deleted");
      push("/shares");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete share";
      toast.error(message);
    } finally {
      deleting = false;
    }
  }

  function copyShareLink() {
    navigator.clipboard.writeText(shareUrl);
    toast.success("Link copied to clipboard");
  }

  async function handleSendNotification() {
    if (!share) return;

    const emailList = notifyEmails
      .split(",")
      .map((e) => e.trim())
      .filter((e) => e.length > 0);

    if (emailList.length === 0) {
      toast.error("Please enter at least one email address");
      return;
    }

    notifying = true;
    try {
      await sharesApi.sendNotification(share.id, emailList);
      toast.success("Notifications sent");
      notifyModal = false;
      notifyEmails = "";
      try {
        recipients = (await sharesApi.getRecipients(share.id)) || [];
      } catch {
        // ignore reload error
      }
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to send notifications";
      toast.error(message);
    } finally {
      notifying = false;
    }
  }
</script>

<div>
  {#if loading}
    <div class="text-center py-16">
      <p class="text-sm text-subtle">Loading...</p>
    </div>
  {:else if share}
    <div class="mb-6">
      <a
        href="#/shares"
        class="text-sm text-muted hover:text-text transition-colors"
      >
        &larr; Back to shares
      </a>
    </div>

    <!-- Share details -->
    <div class="bg-surface rounded-xl border border-border mb-6">
      <div class="p-6">
        {#if editMode}
          <div class="space-y-5">
            <Input label="Name" bind:value={editName} required />
            <div class="space-y-1.5">
              <label
                for="edit-description"
                class="block text-sm font-medium text-text">Description</label
              >
              <textarea
                id="edit-description"
                bind:value={editDescription}
                rows="3"
                class="w-full px-3 py-2 text-sm bg-surface border border-border rounded-lg transition-colors duration-150 placeholder:text-subtle focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-border"
                placeholder="Optional description"
              ></textarea>
            </div>
            <Input
              type="password"
              label="New Password (leave blank to keep current)"
              bind:value={editPassword}
              placeholder="Optional"
              autocomplete="off"
            />
            <Input
              type="number"
              label="Max Downloads"
              bind:value={editMaxDownloads}
              placeholder="Unlimited"
            />
            <Input
              type="number"
              label="Max Views"
              bind:value={editMaxViews}
              placeholder="Unlimited"
            />
            <div class="space-y-1.5">
              <label
                for="edit-expires-at"
                class="block text-sm font-medium text-text">Expires At</label
              >
              <input
                id="edit-expires-at"
                type="date"
                bind:value={editExpiresAt}
                class="w-full px-3 py-2 text-sm bg-surface border border-border rounded-lg transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-border"
              />
            </div>
            <div class="flex gap-2">
              <Button on:click={handleSave} loading={saving}>Save</Button>
              <Button variant="secondary" on:click={() => (editMode = false)}
                >Cancel</Button
              >
            </div>
          </div>
        {:else}
          <div class="flex justify-between items-start">
            <div>
              <h2 class="text-xl font-semibold text-text">{share.name}</h2>
              {#if share.description}
                <p class="text-sm text-muted mt-1">{share.description}</p>
              {/if}
              <div
                class="flex flex-wrap gap-x-4 gap-y-1 mt-3 text-xs text-subtle"
              >
                <span class="font-mono">/{share.slug}</span>
                {#if share.has_password}
                  <span class="inline-flex items-center gap-1">
                    <svg
                      class="w-3 h-3"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke-width="1.5"
                      stroke="currentColor"
                    >
                      <path
                        stroke-linecap="round"
                        stroke-linejoin="round"
                        d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
                      />
                    </svg>
                    Protected
                  </span>
                {/if}
                {#if share.is_reverse_share}
                  <span>Reverse share</span>
                {/if}
              </div>
              <div class="flex gap-4 mt-2 text-xs text-subtle">
                <span
                  >{share.view_count} view{share.view_count !== 1
                    ? "s"
                    : ""}{share.max_views ? ` / ${share.max_views}` : ""}</span
                >
                <span
                  >{share.download_count} download{share.download_count !== 1
                    ? "s"
                    : ""}{share.max_downloads
                    ? ` / ${share.max_downloads}`
                    : ""}</span
                >
                {#if share.expires_at}
                  <span
                    >Expires {new Date(
                      share.expires_at,
                    ).toLocaleDateString()}</span
                  >
                {/if}
              </div>
            </div>
            <div class="flex gap-2 flex-shrink-0 ml-4">
              {#if $emailConfigured}
                <Button
                  variant="secondary"
                  size="sm"
                  on:click={() => (notifyModal = true)}>Send via Email</Button
                >
              {/if}
              <Button
                variant="secondary"
                size="sm"
                on:click={() => (editMode = true)}>Edit</Button
              >
              <Button
                variant="danger"
                size="sm"
                on:click={() => (deleteModal = true)}>Delete</Button
              >
            </div>
          </div>
        {/if}
      </div>

      <!-- Share link -->
      <div class="px-6 py-4 border-t border-border">
        <p
          class="text-xs font-medium text-subtle uppercase tracking-wider mb-2"
        >
          Share Link
        </p>
        <div class="flex gap-2">
          <input
            type="text"
            readonly
            value={shareUrl}
            autocomplete="off"
            class="flex-1 px-3 py-2 text-sm bg-surface-subtle border border-border rounded-lg text-muted"
          />
          <Button variant="secondary" size="sm" on:click={copyShareLink}
            >Copy</Button
          >
        </div>
      </div>

      <!-- Notified recipients -->
      {#if $emailConfigured && recipients.length > 0}
        <div class="px-6 py-4 border-t border-border">
          <p
            class="text-xs font-medium text-subtle uppercase tracking-wider mb-2"
          >
            Notified Recipients
          </p>
          <ul class="space-y-1">
            {#each recipients as r (r.id)}
              <li class="text-sm text-muted">
                {r.email}
                <span class="text-xs text-subtle">
                  - {new Date(r.sent_at).toLocaleDateString()}
                </span>
              </li>
            {/each}
          </ul>
        </div>
      {/if}
    </div>

    <!-- Files -->
    <div class="bg-surface rounded-xl border border-border">
      <div class="p-6">
        <h3 class="text-sm font-semibold text-text mb-4">Files</h3>
        {#if !share.is_reverse_share}
          <div class="mb-4">
            <FileUploader on:files={handleFileUpload} disabled={uploading} />
            {#if uploading}
              <p class="text-xs text-subtle mt-2">Uploading...</p>
            {/if}
          </div>
        {/if}
        <FileList {files} canDelete on:delete={handleFileDelete} />
      </div>
    </div>
  {/if}
</div>

<Modal
  open={deleteModal}
  title="Delete Share"
  on:close={() => (deleteModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to delete "{share?.name}"? This action cannot be
    undone.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (deleteModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={deleting} on:click={handleDeleteShare}
      >Delete</Button
    >
  </div>
</Modal>

<Modal
  open={notifyModal}
  title="Send via Email"
  on:close={() => (notifyModal = false)}
>
  <div class="space-y-4">
    <Input
      label="Recipients"
      bind:value={notifyEmails}
      placeholder="email1@example.com, email2@example.com"
    />
    <div class="flex gap-2 justify-end">
      <Button variant="secondary" on:click={() => (notifyModal = false)}
        >Cancel</Button
      >
      <Button loading={notifying} on:click={handleSendNotification}>Send</Button
      >
    </div>
  </div>
</Modal>
