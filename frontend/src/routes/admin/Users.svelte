<script lang="ts">
  import { onMount } from "svelte";
  import { push, location } from "svelte-spa-router";
  import { Button, Input, Modal } from "../../lib/components";
  import { auth, isAuthenticated, isAdmin, toast } from "../../lib/stores";
  import { api, type User } from "../../lib/api";

  $: usersActive = $location === "/admin/users";
  $: storageActive = $location === "/admin/storage";

  let users: User[] = [];
  let loading = true;

  let createModal = false;
  let creating = false;
  let newEmail = "";
  let newPassword = "";
  let newDisplayName = "";
  let newIsAdmin = false;
  let createErrors: Record<string, string> = {};

  let deleteModal = false;
  let deleting = false;
  let userToDelete: User | null = null;

  $: if ($auth.initialized && !$isAuthenticated) {
    push("/login");
  }

  $: if ($auth.initialized && $isAuthenticated && !$isAdmin) {
    toast.error("Access denied");
    push("/");
  }

  onMount(async () => {
    await loadUsers();
  });

  async function loadUsers() {
    if (!$isAdmin) return;

    loading = true;
    try {
      users = await api.get<User[]>("/admin/users");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to load users";
      toast.error(message);
    } finally {
      loading = false;
    }
  }

  function openCreateModal() {
    newEmail = "";
    newPassword = "";
    newDisplayName = "";
    newIsAdmin = false;
    createErrors = {};
    createModal = true;
  }

  async function handleCreateUser(e: Event) {
    e.preventDefault();
    createErrors = {};

    if (!newEmail.trim()) {
      createErrors = { ...createErrors, email: "Email is required" };
    }
    if (!newDisplayName.trim()) {
      createErrors = {
        ...createErrors,
        displayName: "Display name is required",
      };
    }
    if (!newPassword) {
      createErrors = { ...createErrors, password: "Password is required" };
    } else if (newPassword.length < 8) {
      createErrors = {
        ...createErrors,
        password: "Password must be at least 8 characters",
      };
    }

    if (Object.keys(createErrors).length > 0) {
      return;
    }

    creating = true;
    try {
      const user = await api.post<User>("/admin/users", {
        email: newEmail.trim(),
        password: newPassword,
        display_name: newDisplayName.trim(),
        is_admin: newIsAdmin,
      });
      users = [...users, user];
      createModal = false;
      toast.success("User created");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to create user";
      toast.error(message);
    } finally {
      creating = false;
    }
  }

  function confirmDelete(user: User) {
    userToDelete = user;
    deleteModal = true;
  }

  async function handleDeleteUser() {
    if (!userToDelete) return;

    deleting = true;
    try {
      await api.delete<void>(`/admin/users/${userToDelete.id}`);
      users = users.filter((u) => u.id !== userToDelete!.id);
      deleteModal = false;
      userToDelete = null;
      toast.success("User deleted");
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to delete user";
      toast.error(message);
    } finally {
      deleting = false;
    }
  }

  async function toggleAdmin(user: User) {
    try {
      const updated = await api.patch<User>(`/admin/users/${user.id}`, {
        is_admin: !user.is_admin,
      });
      users = users.map((u) => (u.id === user.id ? updated : u));
      toast.success(
        `${user.display_name} ${updated.is_admin ? "is now" : "is no longer"} an admin`,
      );
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Failed to update user";
      toast.error(message);
    }
  }
</script>

<div class="flex items-center gap-1 mb-6">
  <a
    href="#/admin/users"
    class="px-3 py-1.5 text-sm rounded-md transition-colors {usersActive
      ? 'text-text bg-surface-muted font-medium'
      : 'text-muted hover:text-text hover:bg-surface-subtle'}"
  >
    Users
  </a>
  <a
    href="#/admin/storage"
    class="px-3 py-1.5 text-sm rounded-md transition-colors {storageActive
      ? 'text-text bg-surface-muted font-medium'
      : 'text-muted hover:text-text hover:bg-surface-subtle'}"
  >
    Storage
  </a>
</div>

<div class="flex items-center justify-between mb-6">
  <h2 class="text-lg font-semibold text-text">User Management</h2>
  <Button on:click={openCreateModal}>Create User</Button>
</div>

{#if loading}
  <div class="text-center py-16">
    <p class="text-sm text-subtle">Loading...</p>
  </div>
{:else}
  <div class="bg-surface rounded-xl border border-border overflow-hidden">
    <table class="min-w-full divide-y divide-border">
      <thead>
        <tr class="bg-surface-subtle">
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            User
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Email
          </th>
          <th
            class="px-6 py-3 text-left text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Role
          </th>
          <th
            class="px-6 py-3 text-right text-xs font-medium text-subtle uppercase tracking-wider"
          >
            Actions
          </th>
        </tr>
      </thead>
      <tbody class="divide-y divide-border">
        {#each users as user (user.id)}
          <tr class="hover:bg-surface-subtle transition-colors">
            <td class="px-6 py-4 whitespace-nowrap">
              <span class="text-sm font-medium text-text"
                >{user.display_name}</span
              >
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-sm text-muted">
              {user.email}
            </td>
            <td class="px-6 py-4 whitespace-nowrap">
              <span
                class="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium {user.is_admin
                  ? 'bg-accent text-accent-contrast'
                  : 'bg-surface-muted text-muted'}"
              >
                {user.is_admin ? "Admin" : "User"}
              </span>
            </td>
            <td class="px-6 py-4 whitespace-nowrap text-right text-xs">
              {#if user.id !== $auth.user?.id}
                <button
                  class="text-muted hover:text-text transition-colors mr-3"
                  on:click={() => toggleAdmin(user)}
                >
                  {user.is_admin ? "Remove Admin" : "Make Admin"}
                </button>
                <button
                  class="text-red-500 hover:text-red-700 transition-colors"
                  on:click={() => confirmDelete(user)}
                >
                  Delete
                </button>
              {:else}
                <span class="text-subtle">Current user</span>
              {/if}
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="4" class="px-6 py-8 text-center text-sm text-subtle">
              No users found
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<Modal
  open={createModal}
  title="Create User"
  on:close={() => (createModal = false)}
>
  <form on:submit={handleCreateUser} class="space-y-4">
    <Input
      label="Display Name"
      bind:value={newDisplayName}
      error={createErrors.displayName}
      autocomplete="off"
      required
    />
    <Input
      type="email"
      label="Email"
      bind:value={newEmail}
      error={createErrors.email}
      autocomplete="off"
      required
    />
    <Input
      type="password"
      label="Password"
      bind:value={newPassword}
      placeholder="At least 8 characters"
      error={createErrors.password}
      autocomplete="off"
      required
    />
    <div class="flex items-center gap-2.5">
      <input
        type="checkbox"
        id="newIsAdmin"
        bind:checked={newIsAdmin}
        class="w-4 h-4 text-text border-border rounded focus:ring-accent/20"
      />
      <label for="newIsAdmin" class="text-sm text-muted">Admin privileges</label
      >
    </div>
    <div class="flex gap-2 justify-end pt-2">
      <Button variant="secondary" on:click={() => (createModal = false)}
        >Cancel</Button
      >
      <Button type="submit" loading={creating}>Create</Button>
    </div>
  </form>
</Modal>

<Modal
  open={deleteModal}
  title="Delete User"
  on:close={() => (deleteModal = false)}
>
  <p class="text-sm text-muted mb-5">
    Are you sure you want to delete "{userToDelete?.display_name}"? This action
    cannot be undone.
  </p>
  <div class="flex gap-2 justify-end">
    <Button variant="secondary" on:click={() => (deleteModal = false)}
      >Cancel</Button
    >
    <Button variant="danger" loading={deleting} on:click={handleDeleteUser}
      >Delete</Button
    >
  </div>
</Modal>
