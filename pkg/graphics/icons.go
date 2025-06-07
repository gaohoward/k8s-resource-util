package graphics

import (
	"gioui.org/widget"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

var MenuIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationMenu)
	return icon
}()

var ResIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.FileAttachment)
	return icon
}()

var InfoIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionInfo)
	return icon
}()

var AddIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentAdd)
	return icon
}()

var DeployIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.AVPlayArrow)
	return icon
}()

var AddToResourceIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionNoteAdd)
	return icon
}()

var AddToCollectionIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.AVPlaylistAdd)
	return icon
}()

var DeleteIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionDeleteForever)
	return icon
}()

var RenameIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.EditorModeEdit)
	return icon
}()

var ReloadIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationRefresh)
	return icon
}()

var SaveIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentSave)
	return icon
}()

var UpIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.HardwareKeyboardArrowUp)
	return icon
}()

var DownIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.HardwareKeyboardArrowDown)
	return icon
}()

var CloseIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationClose)
	return icon
}()

var ClearIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentDeleteSweep)
	return icon
}()

var UndeployIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentRemoveCircleOutline)
	return icon
}()

var RefreshIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationRefresh)
	return icon
}()

var ResourceIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionLabel)
	return icon
}()

var ExecIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowForward)
	return icon
}()

var SearchIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionSearch)
	return icon
}()

var ToTopIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.EditorVerticalAlignTop)
	return icon
}()

var ToBottomIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.EditorVerticalAlignBottom)
	return icon
}()

var MoveUpIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowUpward)
	return icon
}()

var MoveDownIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationArrowDownward)
	return icon
}()

var RestoreIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionSettingsBackupRestore)
	return icon
}()

var ReorderIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.NavigationUnfoldMore)
	return icon
}()

var CopyIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentContentCopy)
	return icon
}()

var RunningIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionCheckCircle)
	return icon
}()

var TerminatedIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ContentRemoveCircleOutline)
	return icon
}()

var ErrorIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.AlertErrorOutline)
	return icon
}()

var UnknownIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionInfoOutline)
	return icon
}()

var HelpIcon *widget.Icon = func() *widget.Icon {
	icon, _ := widget.NewIcon(icons.ActionHelpOutline)
	return icon
}()
