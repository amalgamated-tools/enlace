<script lang="ts">
  import { onMount } from "svelte";
  import { Button, Input, FileUploader, FileList } from "../lib/components";
  import { toast } from "../lib/stores";
  import type { Share, FileInfo } from "../lib/api";
  import { importKey, decryptFile, encryptFile } from "../lib/crypto/e2e";

  export let params: { slug: string } = { slug: "" };

  let share: (Share & { is_e2e_encrypted?: boolean }) | null = null;
  let files: FileInfo[] = [];
  let loading = true;
  let error = "";

  let passwordRequired = false;
  let password = "";
  let verifying = false;
  let shareToken = "";

  let uploading = false;

  // E2E encryption state
  let e2eKey: CryptoKey | null = null;
  let e2eKeyMissing = false;
  let decrypting: Record<string, boolean> = {};

  onMount(async () => {
    await loadShare();
  });

  function extractE2EKey(): string | null {
    // The key is in the URL fragment after #key=
    // With svelte-spa-router, the URL format is /#/s/{slug}#key=...
    const fullHash = window.location.hash;
    const keyMatch = fullHash.match(/#key=([A-Za-z0-9_-]+)/);
    return keyMatch ? keyMatch[1] : null;
  }

  async function loadShare() {
    if (!params.slug) return;

    loading = true;
    error = "";
    try {
      const headers: Record<string, string> = {};
      if (shareToken) {
        headers["X-Share-Token"] = shareToken;
      }
      const response = await fetch(`/s/${params.slug}`, { headers });
      const data = await response.json();

      if (response.status === 401 && data.error?.includes("password")) {
        passwordRequired = true;
        loading = false;
        return;
      }

      if (!response.ok || !data.success) {
        error = data.error || "Share not found";
        loading = false;
        return;
      }

      share = data.data.share;
      files = data.data.files || [];

      // If share is E2E encrypted, try to extract the key from URL fragment
      if (share?.is_e2e_encrypted) {
        const keyStr = extractE2EKey();
        if (keyStr) {
          try {
            e2eKey = await importKey(keyStr);
          } catch {
            e2eKeyMissing = true;
          }
        } else {
          e2eKeyMissing = true;
        }
      }
    } catch {
      error = "Failed to load share";
    } finally {
      loading = false;
    }
  }

  async function handlePasswordSubmit(e: Event) {
    e.preventDefault();

    if (!password) {
      toast.error("Please enter the password");
      return;
    }

    verifying = true;
    try {
      const response = await fetch(`/s/${params.slug}/verify`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password }),
      });
      const data = await response.json();

      if (!response.ok || !data.success) {
        toast.error(data.error || "Invalid password");
        return;
      }

      shareToken = data.data.token;
      passwordRequired = false;
      await loadShare();
    } catch {
      toast.error("Failed to verify password");
    } finally {
      verifying = false;
    }
  }

  async function handleFileUpload(event: CustomEvent<File[]>) {
    if (!share) return;

    uploading = true;
    try {
      const formData = new FormData();

      if (share.is_e2e_encrypted && e2eKey) {
        // Encrypt files client-side before upload
        for (const file of event.detail) {
          const encrypted = await encryptFile(e2eKey, file);
          const encryptedBlob = new File([encrypted.blob], file.name, {
            type: "application/octet-stream",
          });
          formData.append("files", encryptedBlob);
          formData.append("encryption_iv", encrypted.iv);
          formData.append("encrypted_metadata", encrypted.encryptedMeta);
        }
      } else {
        event.detail.forEach((file) => formData.append("files", file));
      }

      const response = await fetch(`/s/${params.slug}/upload`, {
        method: "POST",
        body: formData,
      });
      const data = await response.json();

      if (!response.ok || !data.success) {
        toast.error(data.error || "Upload failed");
        return;
      }

      files = [...files, ...data.data];
      toast.success(`${event.detail.length} file(s) uploaded`);
    } catch {
      toast.error("Upload failed");
    } finally {
      uploading = false;
    }
  }

  async function downloadAll() {
    if (!share) return;

    for (const file of files) {
      await downloadFile(file.id);
    }
  }

  async function downloadFile(fileId: string) {
    if (!share) return;

    if (share.is_e2e_encrypted && e2eKey) {
      // Download ciphertext, decrypt in browser, then save
      decrypting = { ...decrypting, [fileId]: true };
      try {
        const file = files.find((f) => f.id === fileId);
        if (!file) return;

        const headers: Record<string, string> = {};
        if (shareToken) {
          headers["X-Share-Token"] = shareToken;
        }

        const response = await fetch(
          `/s/${params.slug}/files/${fileId}`,
          { headers },
        );
        if (!response.ok) {
          toast.error("Failed to download file");
          return;
        }

        const encryptedData = await response.arrayBuffer();

        const decrypted = await decryptFile(
          e2eKey,
          file.encryption_iv || "",
          file.encrypted_metadata || "",
          encryptedData,
        );

        // Trigger browser download of decrypted file
        const url = URL.createObjectURL(decrypted.blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = decrypted.filename;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
      } catch {
        toast.error("Failed to decrypt file");
      } finally {
        decrypting = { ...decrypting, [fileId]: false };
      }
    } else {
      // Standard download
      window.location.href = `/s/${params.slug}/files/${fileId}`;
    }
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
  }
</script>

<div class="min-h-screen bg-surface-subtle">
  <header class="bg-surface border-b border-border">
    <div class="max-w-6xl mx-auto px-6 h-14 flex items-center">
      <span class="text-base font-semibold text-text tracking-tight"
        >enlace</span
      >
    </div>
  </header>

  <main class="max-w-2xl mx-auto px-6 py-10">
    {#if loading}
      <div class="text-center py-16">
        <p class="text-sm text-subtle">Loading...</p>
      </div>
    {:else if error}
      <div class="bg-surface rounded-xl border border-border p-10 text-center">
        <p class="text-sm text-red-500 font-medium">{error}</p>
        <p class="text-sm text-subtle mt-2">
          This share may have expired or been deleted.
        </p>
      </div>
    {:else if passwordRequired}
      <div
        class="bg-surface rounded-xl border border-border p-8 max-w-sm mx-auto"
      >
        <div class="text-center mb-6">
          <div
            class="w-10 h-10 rounded-lg bg-surface-muted flex items-center justify-center mx-auto mb-3"
          >
            <svg
              class="w-5 h-5 text-subtle"
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
          </div>
          <h2 class="text-base font-semibold text-text">Password Required</h2>
          <p class="text-sm text-muted mt-1">
            This share is password protected.
          </p>
        </div>
        <form on:submit={handlePasswordSubmit} class="space-y-4">
          <Input
            type="password"
            label="Password"
            bind:value={password}
            placeholder="Enter password"
            autocomplete="off"
            required
          />
          <Button type="submit" loading={verifying}>
            {verifying ? "Verifying..." : "Continue"}
          </Button>
        </form>
      </div>
    {:else if share}
      <div class="bg-surface rounded-xl border border-border">
        <div class="p-6 border-b border-border">
          <div class="flex items-center gap-2">
            <h2 class="text-xl font-semibold text-text">{share.name}</h2>
            {#if share.is_e2e_encrypted}
              <span
                class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300"
              >
                <svg
                  class="w-3 h-3"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke-width="2"
                  stroke="currentColor"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"
                  />
                </svg>
                E2E Encrypted
              </span>
            {/if}
          </div>
          {#if share.description}
            <p class="text-sm text-muted mt-1">{share.description}</p>
          {/if}
          {#if share.expires_at}
            <p class="text-xs text-subtle mt-2">
              Expires {new Date(share.expires_at).toLocaleDateString()}
            </p>
          {/if}
        </div>

        {#if share.is_e2e_encrypted && e2eKeyMissing}
          <div class="p-6 border-b border-border">
            <div
              class="bg-red-50 dark:bg-red-950/30 border border-red-200 dark:border-red-800 rounded-lg p-4"
            >
              <h3 class="text-sm font-semibold text-red-800 dark:text-red-200 mb-1">
                Encryption Key Required
              </h3>
              <p class="text-xs text-red-700 dark:text-red-300">
                This share is end-to-end encrypted. You need the full share URL
                (including the encryption key) to decrypt these files. The URL
                should contain <code class="font-mono">#key=...</code> at the end.
              </p>
            </div>
          </div>
        {/if}

        {#if share.is_reverse_share}
          <div class="p-6 border-b border-border">
            <h3 class="text-sm font-semibold text-text mb-4">Upload Files</h3>
            <FileUploader on:files={handleFileUpload} disabled={uploading} />
            {#if uploading}
              <p class="text-xs text-subtle mt-2">Uploading...</p>
            {/if}
          </div>
        {/if}

        <div class="p-6">
          <div class="flex justify-between items-center mb-4">
            <h3 class="text-sm font-semibold text-text">Files</h3>
            {#if files.length > 0 && !share.is_reverse_share}
              <Button size="sm" variant="secondary" on:click={downloadAll}
                >Download All</Button
              >
            {/if}
          </div>

          {#if files.length === 0}
            <p class="text-sm text-subtle text-center py-6">
              No files available
            </p>
          {:else}
            <ul
              class="divide-y divide-border border border-border rounded-xl overflow-hidden"
            >
              {#each files as file (file.id)}
                <li
                  class="flex items-center justify-between px-4 py-3 hover:bg-surface-subtle transition-colors"
                >
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
                    <div class="min-w-0">
                      <p class="text-sm font-medium text-text truncate">
                        {file.name}
                      </p>
                      <p class="text-xs text-subtle">
                        {formatSize(file.size)}
                      </p>
                    </div>
                  </div>
                  {#if !share.is_reverse_share}
                    <button
                      class="text-xs text-muted hover:text-text font-medium transition-colors ml-3 flex-shrink-0 disabled:opacity-40 disabled:cursor-not-allowed"
                      disabled={share.is_e2e_encrypted && (e2eKeyMissing || decrypting[file.id])}
                      on:click={() => downloadFile(file.id)}
                    >
                      {#if decrypting[file.id]}
                        Decrypting...
                      {:else}
                        Download
                      {/if}
                    </button>
                  {/if}
                </li>
              {/each}
            </ul>
          {/if}
        </div>
      </div>
    {/if}
  </main>
</div>
