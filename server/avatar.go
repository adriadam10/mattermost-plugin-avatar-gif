package main

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

var (
	errNotLocalStorage = errors.New("file storage is not local")
	errNoAnimated      = errors.New("no animated avatar")
)

// Mattermost re-encodes every uploaded picture as a 128×128 PNG, so the animation is
// lost. setAnimated uploads the first frame the normal way, which updates
// LastPictureUpdate (the cache key clients use) and tells them about it, and then
// writes the GIF over the stored file, which Mattermost serves unchanged.
func (p *Plugin) setAnimated(userID string, data []byte) error {
	animated, firstFrame, err := resizeGIF(data, p.config().maxFrames())
	if err != nil {
		return err
	}
	path, err := p.profileImagePath(userID)
	if err != nil {
		return err
	}
	if current, appErr := p.API.GetProfileImage(userID); appErr == nil && !isGIF(current) {
		if appErr := p.API.KVSet(previousKey(userID), current); appErr != nil {
			return appErr
		}
	}
	if err := p.uploadPicture(userID, firstFrame); err != nil {
		return err
	}
	// Clients ask for the new picture as soon as they hear about it: write it right away.
	return writeFileAtomic(path, animated)
}

// restore puts back the picture the user had before the first GIF.
func (p *Plugin) restore(userID string) error {
	current, appErr := p.API.GetProfileImage(userID)
	if appErr != nil {
		return appErr
	}
	if !isGIF(current) {
		return errNoAnimated
	}
	previous, appErr := p.API.KVGet(previousKey(userID))
	if appErr != nil {
		return appErr
	}
	if previous == nil {
		// Nothing saved (the KV was wiped): keep the first frame.
		_, firstFrame, err := resizeGIF(current, math.MaxInt)
		if err != nil {
			return err
		}
		return p.uploadPicture(userID, firstFrame)
	}
	if err := p.uploadPicture(userID, previous); err != nil {
		return err
	}
	if appErr := p.API.KVDelete(previousKey(userID)); appErr != nil {
		return appErr
	}
	return nil
}

// uploadPicture sets a still picture. Mattermost skips the update, keeping
// LastPictureUpdate and so the clients' cached copy, when the stored file already
// has the same bytes; then a pixel is nudged and it is uploaded again.
func (p *Plugin) uploadPicture(userID string, picture []byte) error {
	before, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return appErr
	}
	if appErr := p.API.SetProfileImage(userID, picture); appErr != nil {
		return appErr
	}
	after, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return appErr
	}
	if after.LastPictureUpdate != before.LastPictureUpdate {
		return nil
	}
	nudged, err := nudgePixel(picture)
	if err != nil {
		return err
	}
	if appErr := p.API.SetProfileImage(userID, nudged); appErr != nil {
		return appErr
	}
	return nil
}

func previousKey(userID string) string {
	return "previous_" + userID
}

// profileImagePath is where the local file store keeps the picture
// (users/<id>/profile.png under FileSettings.Directory). A relative directory is
// relative to the server's working directory, which the bundle path reveals.
func (p *Plugin) profileImagePath(userID string) (string, error) {
	cfg := p.API.GetConfig()
	if cfg.FileSettings.DriverName == nil || *cfg.FileSettings.DriverName != model.ImageDriverLocal {
		return "", errNotLocalStorage
	}
	bundle, err := p.API.GetBundlePath()
	if err != nil {
		return "", err
	}
	dir, err := dataDir(*cfg.FileSettings.Directory, *cfg.PluginSettings.Directory, bundle)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "users", userID, "profile.png"), nil
}

// dataDir resolves the file store directory. The bundle path is the plugins
// directory made absolute by the server, so it gives the server's working directory.
func dataDir(fileDir, pluginsDir, bundle string) (string, error) {
	if filepath.IsAbs(fileDir) {
		return fileDir, nil
	}
	if filepath.IsAbs(pluginsDir) {
		return "", errors.New("cannot find the server directory: plugin directory is absolute")
	}
	suffix := string(filepath.Separator) + filepath.Join(pluginsDir, manifestID)
	root, ok := strings.CutSuffix(filepath.Clean(bundle), suffix)
	if !ok {
		return "", errors.New("cannot find the server directory from " + bundle)
	}
	return filepath.Join(root, fileDir), nil
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
