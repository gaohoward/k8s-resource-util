package panels

import "testing"

func TestConversion(t *testing.T) {
	initContent := "Hello World"
	rootConversion := NewRootConversion(initContent)

	name := rootConversion.GetName()

	if rootConversion.GetSource() != nil {
		t.Errorf("root should not have source: %s", name)
	}

	sourceContent := rootConversion.GetSourceContent()
	// even though the root doesn't have a source
	// it returns its own value to be shown in the panel
	if sourceContent != initContent {
		t.Errorf("root should return its own value: %s", sourceContent)
	}

	if rootConversion.IsReadOnly() {
		t.Errorf("root shouldnt be readonly")
	}

	newVal := "New Value"

	rootConversion.SetValue([]byte(newVal))

	if string(rootConversion.GetValue().content) != newVal {
		t.Errorf("Set value not working: %s", string(rootConversion.GetValue().content))
	}

}
