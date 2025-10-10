package common

// as I didn't find a solution for
// generic copy of over 64k strings
// this is used to handle copy/paste
// internally
type ClipboardHandler interface {
	PasteContent() *string
}

var handler ClipboardHandler

func Copy(h ClipboardHandler) {
	handler = h
}

func HasContent() bool {
	return handler != nil
}

func Paste() *string {
	if handler != nil {
		return handler.PasteContent()
	}
	return nil
}
