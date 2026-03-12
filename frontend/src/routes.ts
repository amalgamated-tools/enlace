import Login from "./routes/Login.svelte";
import Register from "./routes/Register.svelte";
import Dashboard from "./routes/Dashboard.svelte";
import Shares from "./routes/Shares.svelte";
import ShareDetail from "./routes/ShareDetail.svelte";
import NewShare from "./routes/NewShare.svelte";
import SettingsProfile from "./routes/settings/Profile.svelte";
import SettingsSecurity from "./routes/settings/Security.svelte";
import SettingsApiKeys from "./routes/settings/ApiKeys.svelte";
import AdminUsers from "./routes/admin/Users.svelte";
import AdminStorage from "./routes/admin/Storage.svelte";
import AdminEmail from "./routes/admin/Email.svelte";
import AdminWebhooks from "./routes/admin/Webhooks.svelte";
import AdminFileRestrictions from "./routes/admin/FileRestrictions.svelte";
import PublicShare from "./routes/PublicShare.svelte";
import AuthCallback from "./routes/AuthCallback.svelte";
import TwoFactorVerify from "./routes/TwoFactorVerify.svelte";
import TwoFactorSetup from "./routes/TwoFactorSetup.svelte";

export default {
  "/": Dashboard,
  "/login": Login,
  "/register": Register,
  "/auth/callback": AuthCallback,
  "/auth/2fa": TwoFactorVerify,
  "/auth/2fa/setup": TwoFactorSetup,
  "/shares": Shares,
  "/shares/new": NewShare,
  "/shares/:id": ShareDetail,
  "/settings": SettingsProfile,
  "/settings/profile": SettingsProfile,
  "/settings/security": SettingsSecurity,
  "/settings/api-keys": SettingsApiKeys,
  "/admin/users": AdminUsers,
  "/admin/storage": AdminStorage,
  "/admin/email": AdminEmail,
  "/admin/webhooks": AdminWebhooks,
  "/admin/files": AdminFileRestrictions,
  "/s/:slug": PublicShare,
  "*": Dashboard,
};
