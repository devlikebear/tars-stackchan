// Package tts is a zero-dependency Gemini-backed WAV relay for Stack-chan
// remote TTS. It is a Go port of scripts/dev/tts-remote-server.py and keeps
// strict behavioral parity with that server.
package tts

import "strings"

// DefaultGeminiModel mirrors tts-remote-server.py DEFAULT_GEMINI_MODEL.
const DefaultGeminiModel = "gemini-3.1-flash-tts-preview"

// DefaultGeminiEndpointTemplate mirrors DEFAULT_GEMINI_ENDPOINT_TEMPLATE.
const DefaultGeminiEndpointTemplate = "https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent"

// DefaultGeminiVoice mirrors DEFAULT_GEMINI_VOICE.
const DefaultGeminiVoice = "Kore"

// geminiVoices maps a lowercase voice alias to its canonical Gemini name.
// Kept identical to GEMINI_VOICES in tts-remote-server.py.
var geminiVoices = map[string]string{
	"zephyr":        "Zephyr",
	"puck":          "Puck",
	"charon":        "Charon",
	"kore":          "Kore",
	"fenrir":        "Fenrir",
	"leda":          "Leda",
	"orus":          "Orus",
	"aoede":         "Aoede",
	"callirrhoe":    "Callirrhoe",
	"autonoe":       "Autonoe",
	"enceladus":     "Enceladus",
	"iapetus":       "Iapetus",
	"umbriel":       "Umbriel",
	"algieba":       "Algieba",
	"despina":       "Despina",
	"erinome":       "Erinome",
	"algenib":       "Algenib",
	"rasalgethi":    "Rasalgethi",
	"laomedeia":     "Laomedeia",
	"achernar":      "Achernar",
	"alnilam":       "Alnilam",
	"schedar":       "Schedar",
	"gacrux":        "Gacrux",
	"pulcherrima":   "Pulcherrima",
	"achird":        "Achird",
	"zubenelgenubi": "Zubenelgenubi",
	"vindemiatrix":  "Vindemiatrix",
	"sadachbia":     "Sadachbia",
	"sadaltager":    "Sadaltager",
	"sulafat":       "Sulafat",
}

// NormalizeModel mirrors normalize_model: trim, fall back to the default when
// empty.
func NormalizeModel(model string) string {
	model = strings.TrimSpace(model)
	if model == "" {
		return DefaultGeminiModel
	}
	return model
}

// NormalizeVoice mirrors normalize_voice: default when empty, then case-
// insensitive alias lookup, otherwise the trimmed input verbatim.
func NormalizeVoice(voice string) string {
	voice = strings.TrimSpace(voice)
	if voice == "" {
		voice = DefaultGeminiVoice
	}
	if canonical, ok := geminiVoices[strings.ToLower(voice)]; ok {
		return canonical
	}
	return voice
}
