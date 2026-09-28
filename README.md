# AI Tempo

**English** · [Türkçe](README.tr.md)

A macOS menu bar app that tracks the usage limits of your **Claude.ai**,
**Cursor** and **ChatGPT** accounts in one place. Each account gets its own
tab. Each quota card shows **usage percentage**, **time until reset** and
**pace**: whether you are on track to run out before the period ends.

The interface is available in **English** and **Turkish**. By default it
follows your macOS language (Turkish if your system is Turkish, English
otherwise). You can change it in Settings → Language.

> ⚠️ AI Tempo does **not** use official APIs. It calls the same internal
> endpoints that claude.ai, cursor.com and chatgpt.com use in your browser.
> These can change without notice, and session keys/tokens have to be
> refreshed when they expire. Use it only for your own accounts.

## Download

1. Download **`AI-Tempo-macOS.zip`** from the
   [latest release](https://github.com/KaraKunT/AI-Tempo/releases/latest).
   It runs on both Apple Silicon and Intel Macs.
2. Unzip it and move **AI Tempo.app** to your Applications folder.

The app is signed with a Developer ID and notarized by Apple, so it opens
without a security warning.

AI Tempo runs in the menu bar only. It does not appear in the Dock or in
Cmd+Tab.

## Features

- **Menu bar summary.** Every quota with 🟢/🟠/🔴 status, percentage and time
  until reset. Click an account to open its tab.
- **Pace.** The purple marker on each usage bar shows how much of the period
  has passed.
  - `Pace: relaxed`: usage is behind the marker.
  - `Pace: normal`: usage is within ±10 points of it.
  - `Pace: fast ⚠`: usage is ahead of it and you may run out early.
- **Period chart.** Usage from the start of the period until reset, drawn
  against an "ideal pace" line. Filter by Period / Today / Yesterday / Week /
  Month.
- **Query history.** Every query with time, trigger (Auto / Manual / Reset),
  result and duration. Filter by Today / Yesterday / Week / Month.
- **Status in tab titles.** `⟳` means the account is being queried, `⚠` means
  its last query failed.
- **Retry limit.** After 5 consecutive errors an account is no longer queried
  automatically. **Reset Counter** on its tab re-enables it.
- **Codex limit resets** (ChatGPT, optional). Shows available free resets and
  the history of granted and used resets.
- The main and Settings windows reopen where you left them.

## Adding accounts

Open **Settings** from the menu bar or the window, then click **Add Account**.
Each provider needs a session key (and sometimes an ID) copied from your
browser while you are signed in.

The account editor has a **Get from console** button. It gives you a snippet
to paste into the browser's Developer Tools console, which prints the values
in full. Values copied from the Network tab can be silently truncated with
`…`, and a truncated key shows up as "expired".

| Provider | Session key | Organization ID |
|---|---|---|
| **Claude** | `sessionKeyV3` cookie. Developer Tools → **Application → Cookies → claude.ai** | Run the console snippet on claude.ai, or copy the UUID between `/organizations/` and `/usage` in the `…/usage` request URL |
| **Cursor** | `WorkosCursorSessionToken` cookie. **Application → Cookies → cursor.com** (`%3A%3A` is converted to `::` automatically) | Not needed |
| **ChatGPT** | Run the console snippet on chatgpt.com (it copies the token to your clipboard), or copy the `Bearer` token from the `backend-api/wham/usage` request | `chatgpt-account-id` header of the same request (the snippet prints it) |

Claude and Cursor mark their cookies HttpOnly, so JavaScript usually can't
read them. For those two, copy the key from the **Application → Cookies**
panel.

When a key expires, repeat the steps and paste the new key in **Settings →
account → Edit**. If you leave the field empty, the existing key is kept.

## Settings

| Setting | Options |
|---|---|
| Language | Automatic (system language, default), English, Türkçe |
| Auto refresh | 5, 10, 15, 30 minutes · 1, 2, 4, 8, 16, 24 hours |
| Keep history | 2, 7, 14, 35 (default), 60, 90 days. Use at least 35 days to cover monthly periods such as Cursor's |

### Where data is stored

| What | Where |
|---|---|
| Accounts, IDs, settings | `~/Library/Application Support/ai-tempo/settings.json` |
| Session keys / tokens | macOS **Keychain**, service `ai-tempo`. Never written to disk in plain text |
| Query history and chart data | `~/Library/Application Support/ai-tempo/history.db` (SQLite) |

Everything stays on your Mac. AI Tempo only talks to claude.ai, cursor.com
and chatgpt.com.

## Request policy

All queries go through a single scheduler (`internal/usage`), so the window
and the menu bar share results:

- Each account is queried **once** per refresh interval. This is fewer
  requests than an open dashboard tab in a browser makes.
- Accounts are queried **one at a time**, 1.5 s apart.
- A manual refresh of a healthy account runs at most once every 30 s. A
  failed account can be retried right away.
- Claude sits behind Cloudflare, which occasionally blocks non-browser
  requests with HTTP 403. AI Tempo reports this as "Blocked by Cloudflare"
  (not as an expired session) and retries once on a fresh connection.

## Building from source

Requires Go and Xcode Command Line Tools.

```bash
make run       # build and run from the terminal
make package   # build "AI Tempo.app"
make release   # universal (Intel + Apple Silicon), signed + notarized → dist/AI-Tempo-macOS.zip
make release NOTARIZE=0   # sign only (needs a Developer ID certificate)
```

Start at login:

```bash
make login-add      # adds the .app in this folder to login items
make login-remove
```

## Project layout

```
cmd/ai-tempo/        entry point
internal/config/     settings.json, Keychain, migrations
internal/provider/   Claude / Cursor / ChatGPT queries, pace, date helpers
internal/usage/      central query scheduler
internal/history/    SQLite query history and chart samples
internal/i18n/       translations (Turkish source strings → English)
internal/gui/        window, menu bar, settings, charts, theme, icons
```

Dependencies point one way: `gui → usage → provider → config`.

To add a provider, implement the `Provider` interface in a new file under
`internal/provider/`, register it in `init()`, and add its icon in
`internal/gui/icons/` and `providerIcon`. UI strings are written in Turkish
and wrapped in `T(...)`. Add the English translation to
`internal/i18n/en.go`.

## License

[MIT](LICENSE)
