import Login from "./routes/Login.svelte";
import Register from "./routes/Register.svelte";
import Dashboard from "./routes/Dashboard.svelte";
import Shares from "./routes/Shares.svelte";
import ShareDetail from "./routes/ShareDetail.svelte";
import NewShare from "./routes/NewShare.svelte";
import Settings from "./routes/Settings.svelte";
import AdminUsers from "./routes/admin/Users.svelte";
import AdminStorage from "./routes/admin/Storage.svelte";
import AdminEmail from "./routes/admin/Email.svelte";
import PublicShare from "./routes/PublicShare.svelte";
import AuthCallback from "./routes/AuthCallback.svelte";
import TwoFactorVerify from "./routes/TwoFactorVerify.svelte";

export default {
  "/": Dashboard,
  "/login": Login,
  "/register": Register,
  "/auth/callback": AuthCallback,
  "/auth/2fa": TwoFactorVerify,
  "/shares": Shares,
  "/shares/new": NewShare,
  "/shares/:id": ShareDetail,
  "/settings": Settings,
  "/admin/users": AdminUsers,
  "/admin/storage": AdminStorage,
  "/admin/email": AdminEmail,
  "/s/:slug": PublicShare,
  "*": Dashboard,
};
