# Scrubadubber - Mac Tester Guide

Thanks for helping test Scrubadubber on macOS! This takes about **10 minutes**.
You don't need to be technical - just follow the steps and tell us what you see.

Scrubadubber runs quietly in your menu bar and protects the data your AI coding
tools (like Claude Code) send out. Once it's set up, you use `claude` exactly as
before - nothing in your workflow changes.

---

## What you need

- A Mac running **macOS 11 (Big Sur) or newer**. Both Apple Silicon (M1/M2/M3/M4)
  and Intel Macs are supported.
- The download link we sent you (a file named **`scrubadubber.dmg`**). If you
  weren't sent a direct link, get the latest one here:
  https://github.com/salehkreiner/scrubadubber/releases
- About 10 minutes.

---

## Step 1 - Download and open the installer

1. Download **`scrubadubber.dmg`**.
2. Double-click it. A window opens showing the **Scrubadubber** app and a shortcut
   to your **Applications** folder.
3. **Drag the Scrubadubber icon onto the Applications folder** in that window.

That copies the app to your Mac. You can now close the window and eject the disk
image (drag it to the Trash / click the eject arrow in Finder).

---

## Step 2 - Open Scrubadubber for the first time

Scrubadubber isn't yet registered with Apple (that's a planned upgrade), so macOS
will block the **first** launch to be cautious. This is expected. Here's how to get
past it:

1. Open your **Applications** folder (in Finder, press `Shift+Cmd+A`).
2. **Right-click** (or hold `Control` and click) the **Scrubadubber** app, then
   choose **Open**.
3. A warning appears saying macOS can't verify the developer. Click **Open** again.

> If instead you see **"Scrubadubber is damaged and can't be opened"**, don't drag
> it to the Trash. Take a screenshot and send it to us - and note it in your report
> below. (There's a one-line fix we can give you.)

You only have to do this once. After the first open, it launches normally.

---

## Step 3 - Watch the menu bar icon

Look at the **top-right of your screen**, in the row of small icons next to the
clock and Wi-Fi. A small **Scrubadubber circle** appears there. Its color tells you
the status:

| Color | Meaning |
|-------|---------|
| **Grey** | Just starting up. |
| **Green** | All good - you're protected. |
| **Yellow** | Running, but not fully set up yet. |
| **Red** | Something's wrong - the protection service isn't running. |

On the very first launch, Scrubadubber downloads a couple of helper components in
the background, so the icon may sit on **grey** for **up to a minute**, then turn
**green**. Please give it a minute before deciding anything is wrong.

Click the icon to see the menu. When healthy it shows **"Protected - Claude Code"**.

---

## Step 4 - Try it out

1. Open a **brand-new Terminal window** (Finder -> Applications -> Utilities ->
   Terminal). It must be a new window opened **after** Scrubadubber turned green,
   so it picks up the new settings.
2. Run `claude` the way you normally would.
3. It should behave exactly as before. Behind the scenes, your traffic is now
   protected.

---

## What success looks like

- [ ] The DMG opened and you dragged the app to Applications.
- [ ] Scrubadubber opened after the right-click -> Open step.
- [ ] The menu bar icon turned **green** within a minute or two.
- [ ] The menu shows **"Protected - Claude Code"**.
- [ ] `claude` runs normally in a new Terminal window.

If all five are true: **you're done - thank you!** Just reply "all green, works."

---

## If something goes wrong

Please send us a quick report. Even a partial one helps a lot. Include:

1. **Your Mac:** Apple Silicon or Intel? (Apple menu -> About This Mac.)
2. **Your macOS version** (same window, e.g. "macOS 14 Sonoma").
3. **Where it stopped** - which step number above, and what happened.
4. **The exact message** you saw (a screenshot is perfect).
5. **The icon color** it ended up on (grey / green / yellow / red), and whether you
   waited a minute or two.
6. **The log files** (see below).

### How to grab the logs

Copy this line, paste it into Terminal, and press Return. It opens the folder with
the log files in Finder:

```bash
open ~/Library/Application\ Support/scrubadubber/logs
```

You'll see **`app.log`** and **`hub.log`**. Drag both into your reply (email or
chat). If that folder doesn't exist, tell us - that itself is useful information.

---

## Optional - start over from scratch

Only if you hit a bad state and want a clean retry, or we ask you to. Copy-paste
each line into Terminal and press Return:

```bash
# 1. Quit Scrubadubber first: click its menu bar icon, then choose Quit.

# 2. Remove the login item, app data, and the app itself.
launchctl bootout gui/$(id -u) ~/Library/LaunchAgents/com.scrubadubber.app.plist 2>/dev/null
rm -f ~/Library/LaunchAgents/com.scrubadubber.app.plist
rm -rf ~/Library/Application\ Support/scrubadubber
rm -rf /Applications/Scrubadubber.app
```

This leaves a small setup block in your shell profile (`~/.zshrc`); it's harmless,
but tell us if you'd like the exact lines to remove. Then you can start again from
Step 1.

---

Thank you for testing! Your report - success **or** failure - is exactly what we
need to make the Mac version solid.
