package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

const (
	manifestID = "com.github.adriadam10.avatar-gif"
	command    = "avatar-gif"
)

type Plugin struct {
	plugin.MattermostPlugin
	configuration atomic.Pointer[configuration]
}

func (p *Plugin) OnActivate() error {
	return p.API.RegisterCommand(&model.Command{
		Trigger:          command,
		AutoComplete:     true,
		AutoCompleteHint: "[URL | off]",
		AutoCompleteDesc: "Animated GIF profile picture: pick a file, give a GIF URL, or 'off' to remove it.",
		DisplayName:      "Animated Avatars",
	})
}

func (p *Plugin) OnConfigurationChange() error {
	var raw rawConfiguration
	if err := p.API.LoadPluginConfiguration(&raw); err != nil {
		return err
	}
	p.configuration.Store(raw.parse())
	return nil
}

func (p *Plugin) config() *configuration {
	if c := p.configuration.Load(); c != nil {
		return c
	}
	return defaultConfiguration()
}

func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	arg := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(args.Command), "/"+command))
	t := p.translator(args.UserId)
	var msg string
	switch {
	case arg == "":
		// The webapp opens a file picker instead of sending this; the mobile apps cannot.
		msg = t("usage")
	case arg == "off":
		msg = p.result(t, p.restore(args.UserId), "removed")
	default:
		data, err := fetchGIF(context.Background(), strings.Trim(arg, "<>"), p.config().maxBytes())
		if err == nil {
			err = p.setAnimated(args.UserId, data)
		}
		msg = p.result(t, err, "done")
	}
	p.API.SendEphemeralPost(args.UserId, &model.Post{UserId: args.UserId, ChannelId: args.ChannelId, RootId: args.RootId, Message: msg})
	return &model.CommandResponse{}, nil
}

// ServeHTTP takes the GIF chosen in the webapp's file picker: POST /api/v1/avatar,
// multipart with "image", "channel_id" and "root_id". The result goes to the
// channel as an ephemeral post, like the command's.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-Id")
	if userID == "" {
		http.Error(w, "not logged in", http.StatusUnauthorized)
		return
	}
	if r.URL.Path != "/api/v1/avatar" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	maxBytes := p.config().maxBytes()
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+64<<10)
	err := p.receive(r, userID, maxBytes)
	t := p.translator(userID)
	p.API.SendEphemeralPost(userID, &model.Post{
		UserId:    userID,
		ChannelId: r.FormValue("channel_id"),
		RootId:    r.FormValue("root_id"),
		Message:   p.result(t, err, "done"),
	})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
	}
}

func (p *Plugin) receive(r *http.Request, userID string, maxBytes int64) error {
	file, header, err := r.FormFile("image")
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			return errFileTooBig
		}
		return err
	}
	defer file.Close()
	if header.Size > maxBytes {
		return errFileTooBig
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	return p.setAnimated(userID, data)
}

// result turns an error into a message for the user, logging the unexpected ones.
func (p *Plugin) result(t func(string) string, err error, ok string) string {
	for _, known := range []struct {
		err error
		key string
	}{
		{errNotGIF, "not_gif"},
		{errNotAnimated, "not_animated"},
		{errTooManyFrames, "too_many_frames"},
		{errTooLarge, "too_large"},
		{errFileTooBig, "too_big"},
		{errBadURL, "bad_url"},
		{errPrivateHost, "bad_url"},
		{errNoAnimated, "nothing_to_remove"},
		{errNotLocalStorage, "storage"},
	} {
		if errors.Is(err, known.err) {
			return t(known.key)
		}
	}
	if err != nil {
		p.API.LogWarn("Animated avatar failed", "error", err.Error())
		return t("failed") + " (" + err.Error() + ")"
	}
	return t(ok)
}

func main() {
	plugin.ClientMain(&Plugin{})
}
