package main

import "strings"

var messages = map[string]map[string]string{
	"es": {
		"usage":             "Escribe `/avatar-gif` en el navegador o en la app de escritorio para elegir un GIF, `/avatar-gif <URL>` con un enlace a un GIF (vale uno de Giphy o Tenor) o `/avatar-gif off` para volver a tu foto de antes.",
		"done":              "Listo: tu foto de perfil es ahora el GIF. Quien la tenga en caché puede tardar un poco en verla animada.",
		"removed":           "Listo: has vuelto a tu foto de perfil de antes.",
		"not_gif":           "Eso no es un GIF. Si es un enlace, pon el del GIF (o la página de Giphy o Tenor).",
		"not_animated":      "Ese GIF no está animado: para una foto fija usa el cambio de foto normal del perfil.",
		"too_many_frames":   "El GIF tiene demasiados fotogramas.",
		"too_large":         "El GIF es demasiado grande (más de 2000×2000 píxeles).",
		"too_big":           "El fichero pasa del tamaño máximo permitido.",
		"bad_url":           "Solo valen enlaces `https://` a sitios públicos.",
		"nothing_to_remove": "Tu foto de perfil no es un GIF: no hay nada que quitar.",
		"storage":           "Este servidor no guarda los ficheros en disco local, así que los avatares animados no funcionan aquí.",
		"failed":            "No se ha podido poner el GIF",
	},
	"en": {
		"usage":             "Type `/avatar-gif` in the browser or the desktop app to pick a GIF, `/avatar-gif <URL>` with a link to a GIF (a Giphy or Tenor page works), or `/avatar-gif off` to go back to your previous picture.",
		"done":              "Done: your profile picture is now the GIF. Anyone with it cached may take a while to see it animated.",
		"removed":           "Done: your previous profile picture is back.",
		"not_gif":           "That is not a GIF. For a link, use the GIF itself (or its Giphy or Tenor page).",
		"not_animated":      "That GIF is not animated: use the normal profile picture change for a still picture.",
		"too_many_frames":   "The GIF has too many frames.",
		"too_large":         "The GIF is too large (over 2000×2000 pixels).",
		"too_big":           "The file is over the size limit.",
		"bad_url":           "Only `https://` links to public sites are allowed.",
		"nothing_to_remove": "Your profile picture is not a GIF: there is nothing to remove.",
		"storage":           "This server does not keep files on local disk, so animated avatars do not work here.",
		"failed":            "Could not set the GIF",
	},
}

func (p *Plugin) translator(userID string) func(string) string {
	lang := "en"
	if user, appErr := p.API.GetUser(userID); appErr == nil && strings.HasPrefix(user.Locale, "es") {
		lang = "es"
	}
	return func(key string) string { return messages[lang][key] }
}
