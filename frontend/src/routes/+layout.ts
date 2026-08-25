// A pure client-rendered SPA: there is no Node server in the deployment, only
// the Go binary serving the compiled shell, so nothing may run server-side and
// nothing can be prerendered (every route is behind an auth check).
export const ssr = false;
export const prerender = false;
