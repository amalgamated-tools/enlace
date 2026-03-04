<script lang="ts">
  import { push } from "svelte-spa-router";
  import { Button, Input, FileUploader } from "../lib/components";
  import { auth, isAuthenticated, toast } from "../lib/stores";
  import { sharesApi, filesApi } from "../lib/api";

  let name = "";
  let description = "";
  let slug = "";
  let password = "";
  let maxDownloads: string = "";
  let maxViews: string = "";
  let expiresAt = "";
  let isReverseShare = false;

  let recipients = "";
  let pendingFiles: File[] = [];
  let creating = false;
  let errors: Record<string, string> = {};

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  function handleFileSelect(event: CustomEvent<File[]>) {
    pendingFiles = [...pendingFiles, ...event.detail];
  }

  function removeFile(index: number) {
    pendingFiles = pendingFiles.filter((_, i) => i !== index);
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();
    errors = {};

    if (!name.trim()) {
      errors = { ...errors, name: "Name is required" };
    }

    if (!isReverseShare && pendingFiles.length === 0) {
      errors = { ...errors, files: "Please add at least one file" };
    }

    if (Object.keys(errors).length > 0) {
      return;
    }

    creating = true;
    try {
      const recipientList = recipients
        .split(",")
        .map((e) => e.trim())
        .filter((e) => e.length > 0);

      const share = await sharesApi.create({
        name: name.trim(),
        description: description.trim() || undefined,
        slug: slug.trim() || undefined,
        password: password || undefined,
        max_downloads: maxDownloads ? parseInt(maxDownloads, 10) : undefined,
        max_views: maxViews ? parseInt(maxViews, 10) : undefined,
        expires_at: expiresAt || undefined,
        is_reverse_share: isReverseShare,
        recipients: recipientList.length > 0 ? recipientList : undefined,
      });

      if (pendingFiles.length > 0) {
        await filesApi.upload(share.id, pendingFiles);
      }

      toast.success("Share created successfully");
      push(`/shares/${share.id}`);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create share";
      toast.error(message);
    } finally {
      creating = false;
    }
  }
</script>

<div>
  <div class="mb-6">
    <a
      href="#/shares"
      class="text-sm text-muted hover:text-text transition-colors"
    >
      &larr; Back to shares
    </a>
  </div>

  <h2 class="text-lg font-semibold text-text mb-6">Create New Share</h2>

  <div class="bg-surface rounded-xl border border-border p-6">
    <form on:submit={handleSubmit} class="space-y-6">
      <Input
        label="Name"
        bind:value={name}
        placeholder="My Share"
        error={errors.name}
        required
      />

      <div class="space-y-1.5">
        <label
          for="new-share-description"
          class="block text-sm font-medium text-text">Description</label
        >
        <textarea
          id="new-share-description"
          bind:value={description}
          rows="3"
          class="w-full px-3 py-2 text-sm bg-surface border border-border rounded-lg transition-colors duration-150 placeholder:text-subtle focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-border"
          placeholder="Optional description"
        ></textarea>
      </div>

      <Input
        label="Custom Slug"
        bind:value={slug}
        placeholder="my-custom-slug (optional)"
      />

      <Input
        type="password"
        label="Password Protection"
        bind:value={password}
        placeholder="Optional password"
        autocomplete="off"
      />

      <div class="grid grid-cols-2 gap-4">
        <Input
          type="number"
          label="Max Downloads"
          bind:value={maxDownloads}
          placeholder="Unlimited"
        />
        <Input
          type="number"
          label="Max Views"
          bind:value={maxViews}
          placeholder="Unlimited"
        />
      </div>

      <div class="space-y-1.5">
        <label
          for="new-share-expires-at"
          class="block text-sm font-medium text-text">Expires At</label
        >
        <input
          id="new-share-expires-at"
          type="date"
          bind:value={expiresAt}
          class="w-full px-3 py-2 text-sm bg-surface border border-border rounded-lg transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-border"
        />
      </div>

      <Input
        label="Notify by Email"
        bind:value={recipients}
        placeholder="email1@example.com, email2@example.com (optional)"
      />

      <div class="flex items-center gap-2.5">
        <input
          type="checkbox"
          id="isReverseShare"
          bind:checked={isReverseShare}
          class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
        />
        <label for="isReverseShare" class="text-sm text-muted">
          Reverse share (allow others to upload files)
        </label>
      </div>

      {#if !isReverseShare}
        <div>
          <p class="text-sm font-medium text-text mb-2">
            Files {#if errors.files}<span class="text-red-500"
                >- {errors.files}</span
              >{/if}
          </p>
          <FileUploader on:files={handleFileSelect} />
          {#if pendingFiles.length > 0}
            <ul
              class="mt-4 divide-y divide-border border border-border rounded-xl overflow-hidden"
            >
              {#each pendingFiles as file, index (index)}
                <li class="flex items-center justify-between px-4 py-3">
                  <div class="flex items-center gap-3 min-w-0">
                    <div
                      class="w-8 h-8 rounded-lg bg-surface-muted flex items-center justify-center flex-shrink-0"
                    >
                      <svg
                        class="w-4 h-4 text-subtle"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke-width="1.5"
                        stroke="currentColor"
                      >
                        <path
                          stroke-linecap="round"
                          stroke-linejoin="round"
                          d="M19.5 14.25v-2.625a3.375 3.375 0 00-3.375-3.375h-1.5A1.125 1.125 0 0113.5 7.125v-1.5a3.375 3.375 0 00-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 00-9-9z"
                        />
                      </svg>
                    </div>
                    <span class="text-sm text-text truncate"
                      >{file.name}</span
                    >
                  </div>
                  <button
                    type="button"
                    class="text-xs text-subtle hover:text-red-500 transition-colors ml-3 flex-shrink-0"
                    on:click={() => removeFile(index)}
                  >
                    Remove
                  </button>
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}

      <div class="flex gap-2 pt-2">
        <Button type="submit" loading={creating}>
          {creating ? "Creating..." : "Create Share"}
        </Button>
        <Button variant="secondary" on:click={() => push("/shares")}
          >Cancel</Button
        >
      </div>
    </form>
  </div>
</div>
