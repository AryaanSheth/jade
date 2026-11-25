# Jade - Just A Discord Embedder

Jade is a standalone Go web service for rich Discord link previews. **It is not a Discord bot.** It needs no Discord application, bot token, gateway connection, message permissions, or server invitation.

Paste a public Instagram, X/Twitter, or TikTok post into Jade's converter, then share the resulting Jade URL in Discord. Discord fetches server-rendered metadata from Jade and displays each photo or video as its own item in a native Media Gallery.

## Primary platforms

| Platform | Media handled by the adapters |
| --- | --- |
| Instagram | Public single photos, reels/videos, photo carousels, and mixed photo/video posts |
| X / Twitter | Public photo posts, videos, GIF videos, and mixed media from the linked post |
| TikTok | Public videos and photo-mode slideshows, including `/photo/` URLs and short share links |

YouTube and Reddit are future extensions. They are not advertised as working adapters in this version.

Public accessibility in a browser does not guarantee that a platform exposes the post to a hosting provider. The adapters read public Instagram post metadata, TikTok hydration JSON, and X's syndication response. These are unofficial, changeable interfaces. Instagram and TikTok can return login walls or block datacenter requests. Jade reports an error when complete supported media cannot be extracted; it does not substitute a cover image and call that a full carousel. Live compatibility must be checked after deployment with representative posts.

## Slides inside Discord

- A post's images remain separate, in source order. Videos have separate play controls.
- Discord permits **10 gallery items total per embed**, with a **3,000-byte component payload**. Jade fits both limits. A very long hostname or source URL can reduce the number of slides per set.
- Longer posts get `?page=2`, `?page=3`, etc. share URLs. The post's web page shows every extracted item and the converter exposes each share set.
- Link buttons open a web page; they do **not** update an existing Discord message. Share the next set's URL to create another native embed. Bot-free previews cannot bypass Discord's 10-item limit.
- Standard Open Graph / Twitter tags remain as a first-item fallback when component embeds aren't displayed.
- There is a bounded 100-item service limit. Posts exceeding it fail explicitly rather than silently truncating.

## URL examples

Replace `jade.example.com` with your deployment hostname, including a free `your-project.vercel.app` hostname.

```text
https://jade.example.com/instagram/p/POST_CODE/
https://jade.example.com/tiktok/@creator/photo/1234567890123456789
https://jade.example.com/x/i/status/1234567890123456789
https://jade.example.com/instagram.com/p/POST_CODE/
https://jade.example.com/https://www.instagram.com/p/POST_CODE/
```

The converter removes source tracking parameters. An X `/photo/N` or `/video/N` link normalizes to the full post so its other media is included.

An arbitrary prefix is not DNS magic: `kkinstagram.com` only works if you own and configure that domain. With a free Vercel hostname, use the path forms above.

## Run locally

Requires Go 1.24.4 or newer. The server uses the Go standard library only; there is no Python, Node build, browser automation, or external embed-fixer dependency.

```sh
go run .
```

Open http://localhost:8080. Set `PORT` to change the port and set `JADE_PUBLIC_URL` to the corresponding origin. A local instance is not reachable by Discord until publicly hosted over HTTPS. `.env.example` documents settings; the binary reads environment variables, not dotenv files.

In production, set `JADE_SIGNING_SECRET` to a permanent random value of at least 32 characters. This signs media links without storing a database. Changing it invalidates asset links already cached by Discord. Local runs generate a temporary secret if one isn't configured.

## Free hosting: Vercel

GitHub Pages only serves static files; it cannot fetch posts or generate per-post server-rendered embeds. Vercel's Go framework runs the root `main.go` server directly. `vercel.json` selects `"framework": "go"`; the Go builder supplies routing automatically. Do not add a `functions` entry for `api/index.go` or rewrite every request to that file: the Go framework ignores legacy Go API functions and that mapping causes an `unused_function` build error. The existing `api` adapter is retained for compatibility tests and is not the production entry point.

1. Import this GitHub repository into a personal Vercel project. Use the **Go** framework preset, as specified in `vercel.json`. Keep the repository root as the Root Directory and leave Build Command, Install Command and Output Directory overrides off. The Go framework compiles and runs `main.go` and uses its `PORT` listener.
2. Set `JADE_SIGNING_SECRET` to a random secret of at least 32 characters.
3. Set `JADE_PUBLIC_URL=https://YOUR-PROJECT.vercel.app` after your project hostname is assigned. If omitted, Jade uses Vercel's production hostname environment variable, falling back to the deployment hostname.
4. Keep `JADE_MEDIA_MODE=redirect` for the free serverless setup. Redeploy after changing environment variables.
5. Make the production deployment publicly reachable. A login gate or interactive bot challenge will prevent Discord from fetching it.
6. Open the converter, check representative photo, video, and mixed posts, then test the generated links in Discord and https://discord.com/developers/embeds.

Vercel deployment and live Discord rendering are not implied by a successful build. Verify rewrites, cold-start response time, media redirects and actual platform access on the deployed hostname.

### Media delivery and free-plan limits

In `redirect` mode, signed stable asset links resolve the post and return a fresh redirect to the original platform CDN. Original media bytes do not flow through the Go function, avoiding large function response bodies and video bandwidth usage. CDN hotlink restrictions can still cause a specific image/video to fail. Redirect mode cannot add request headers to Discord's subsequent CDN request.

For a persistent Go server, `JADE_MEDIA_MODE=proxy` streams media through Jade with GET/HEAD and byte-range support. It limits images to 8 MiB and videos to 50 MiB and restricts CDN destinations. Large media proxying on serverless hosts needs separate validation against that host's runtime limits; it is not the default Vercel path.

Vercel Hobby is intended for personal, non-commercial projects and has usage limits. A supplied `.vercel.app` domain avoids buying a domain; it does not promise unlimited requests, uptime, scraping success, or free commercial operation. Rate limits and the metadata cache are per process and therefore per warm instance on serverless hosting.

## Configuration

| Variable | Purpose |
| --- | --- |
| `JADE_PUBLIC_URL` | Canonical public HTTPS origin for generated share/media links |
| `JADE_SIGNING_SECRET` | Permanent random signing secret, at least 32 characters |
| `JADE_MEDIA_MODE` | `redirect` on Vercel; `proxy` by default on a persistent server |
| `JADE_INSTAGRAM_DOC_ID` | Configurable Instagram public query document ID |
| `PORT` | Standalone listener port, default `8080` |

Requests have bounded metadata size, time, concurrency, cache and item count. Only approved platform/CDN hosts are fetched. DNS answers are checked for private/reserved addresses and the selected addresses are pinned to connections. Redirects are revalidated. Tokens contain a canonical source post, not an arbitrary upstream URL. No user browser cookies are collected. No message contents or source URLs are logged by the application.

## Validate

```sh
gofmt -w .
go test -race ./...
go vet ./...
go build ./...
```

Tests cover public URL normalization, photo-only and mixed carousels, TikTok hydration data, media order, complete extraction failures, native gallery JSON, pagination boundaries, escaping, signed assets, private-network rejection, and cache coalescing. Fixtures are deterministic; they do not prove current live access to the platforms. GitHub Actions runs the Go checks and frontend syntax check.

## References

- [Discord component link previews](https://github.com/discord/discord-api-docs/blob/main/developers/link-previews/component-embeds.mdx)
- [Vercel Go backend support](https://vercel.com/changelog/zero-configuration-go-backend-support)
- [Vercel function limits](https://vercel.com/docs/functions/limitations)
- [Vercel Hobby plan](https://vercel.com/docs/plans/hobby)
- [GitHub Pages limitations](https://docs.github.com/en/pages/getting-started-with-github-pages/creating-a-github-pages-site)

The previous Discord bot implementation has been replaced; its old bot setup instructions no longer apply.
