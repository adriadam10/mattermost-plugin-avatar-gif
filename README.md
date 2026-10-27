# Mattermost Animated Avatars plugin

Animated GIF profile pictures in Mattermost, which otherwise turns every uploaded picture into a still PNG.

| Command | What it does |
|---|---|
| `/avatar-gif` | Opens a file picker (browser and desktop app) and sets the chosen GIF |
| `/avatar-gif <URL>` | Downloads the GIF and sets it; a Giphy or Tenor page link works too (its `og:image`) |
| `/avatar-gif off` | Puts back the picture you had before the first GIF |

The GIF is cropped to a centred square and scaled to 128×128, keeping every frame and its timing.
The reply is an ephemeral message in the channel, in Spanish or English depending on the user's language.

## How it works

Mattermost re-encodes every profile picture as a 128×128 PNG, but serves the stored file
(`users/<id>/profile.png` in the file store) as it is. So the plugin:

1. Saves the current picture in the plugin's KV store, unless it is already a GIF.
2. Uploads the first frame with the plugin API (`SetProfileImage`). That updates
   `LastPictureUpdate`, the cache key in the picture URL, and tells every client.
3. Writes the resized GIF over the stored file right away. Browsers decode images by
   their content, so the GIF is animated even though it is served as `image/png`.

If the stored file already had the same bytes, Mattermost would not update
`LastPictureUpdate` and clients would keep their cached picture, so the plugin then
uploads the frame again with one pixel changed by one.

## Limitations

- Only works with local file storage (`FileSettings.DriverName = local`); with S3 the
  command says so and changes nothing.
- Clients ask for the new picture as soon as they are told about it, so one could
  still fetch the still frame in the moment between steps 2 and 3, and keep it
  cached for up to a day.
- Changing the picture the normal way replaces the GIF, as expected.
- If the user had the default picture (initials), `/avatar-gif off` puts back that
  image as an uploaded picture, so it will not follow later name changes.
- The file picker is a webapp feature: in the mobile apps use `/avatar-gif <URL>`.

## Settings

| Setting | Default | |
|---|---|---|
| Maximum GIF size (MB) | 5 | Largest file accepted, before resizing |
| Maximum frames | 200 | GIFs with more frames are rejected |

URLs must be `https://` and resolve to public addresses, so the server cannot be used to reach internal services.

## Build

```sh
./build.sh   # tests and builds dist/com.github.adriadam10.avatar-gif-<version>.tar.gz (only needs Docker)
```

Releases are published by CI when a `vX.Y.Z` tag matching `version` in `plugin.json` is pushed.

## Install

System Console → Plugins → Plugin Management → upload the `.tar.gz` from the
latest release, or with `mmctl plugin install-url <release asset URL>`.
Requires Mattermost 12.0 or later.
