package plugin_volume

import (
	"image/color"
	"os"
	"runtime"
	"script_server/commands"
	"script_server/utils"
	"strconv"
	"time"

	"github.com/gopxl/pixel"
	"github.com/gopxl/pixel/imdraw"
	"github.com/gopxl/pixel/pixelgl"
	"github.com/gopxl/pixel/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

type volumeBarCommand int

const (
	vbCommandInitWindowAfterSettings volumeBarCommand = iota
	vbCommandUpdateVolume
	vbCommandCloseWindow
)

type volumeBar struct {
	win       *pixelgl.Window
	commands  chan volumeBarCommand
	rectImage *imdraw.IMDraw
	myFont    *text.Atlas
}

var globalVb = &volumeBar{
	commands:  make(chan volumeBarCommand, 5),
	rectImage: imdraw.New(nil),
}

func init() {
	go func() {
		runtime.LockOSThread()
		pixelgl.Run(globalVb.init)
	}()
	commands.AddCloseFunc("Volume", func() { globalVb.PushCommand(vbCommandCloseWindow) })
}

func (vb *volumeBar) init() {
	//Create the window
	cfg := pixelgl.WindowConfig{
		Title:                  "Volume percentage",
		Bounds:                 pixel.R(0, 0, 400, 400),
		Undecorated:            true,
		Resizable:              false,
		TransparentFramebuffer: true,
		AlwaysOnTop:            true,
		Invisible:              true,
		VSync:                  true,
	}
	if _win, err := pixelgl.NewWindow(cfg); err != nil {
		panic(err)
	} else {
		vb.win = _win
	}

	vb.runWindowLoop()
}

func (vb *volumeBar) runWindowLoop() {
	//Current window state
	type windowState int
	const (
		windowStateHidden windowState = iota
		windowStateInitializing
		windowStateVisible
	)
	currentWindowState := windowStateHidden

	//Set up the timer for hiding the window
	myTimer := time.AfterFunc(0, func() {
		vb.win.Hide()
		currentWindowState = windowStateHidden
	})

	//Main loop for the window
	for !vb.win.Closed() {
		//Either get a command or update the window and try again
		var vbCommand volumeBarCommand
		select {
		case vbCommand = <-vb.commands:
			break
		default:
			vb.win.Update()
			continue
		}

		//Process a volume bar command
		switch vbCommand {
		case vbCommandInitWindowAfterSettings:
			vb.initWindowAfterSettings()
		case vbCommandUpdateVolume:
			//Do not show the window until it has been positioned
			switch currentWindowState {
			case windowStateHidden:
				//Position the window
				vb.win.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 0}) //Make the window invisible until it has been positioned
				globalXWO.HideFromTaskbar()
				vb.win.Show()
				vb.win.SetPos(pixel.Vec{
					X: float64(vs.VolumeBarLeftOffset + (vs.ScreenWidth-(vs.PercentPixelWidth*vs.OverMaxVolumeMax))/2),
					Y: float64(vs.VolumeBarTop),
				})

				//Wait 200ms to show the window
				currentWindowState = windowStateInitializing
				go func() {
					time.Sleep(200 * time.Millisecond)
					currentWindowState = windowStateVisible
					vb.Update()
				}()
			case windowStateInitializing:
				//If still initializing nothing to do
			case windowStateVisible:
				//Ready to update the window
				globalXWO.SetOnTop()
				globalXWO.MakeNonFocusable()
				vb.drawWindow()
				myTimer.Reset(time.Duration(vs.VolumeBarTimeout) * time.Millisecond) //Hide the window after a timeout
			}
		case vbCommandCloseWindow:
			return
		default:
			utils.PrintError("Invalid volume bar command")
		}

		vb.win.Update()
	}
}

// Draw the window
func (vb *volumeBar) drawWindow() {
	//Draw the 3 different parts of the volume bar
	vb.win.Clear(color.RGBA{R: 0, G: 0, B: 0, A: 0}) //Start with a completely transparent window
	currentVolume := globalVP.GetCurrentVolume()
	if currentVolume > 0 { //Normal volume bar
		vb.drawBarPart(vs.VolColor, 0, min(currentVolume, vs.NormalVolumeMax))
	}
	if currentVolume > vs.NormalVolumeMax { //Over-max volume bar
		vb.drawBarPart(vs.OverMaxColor, vs.NormalVolumeMax, currentVolume)
	}
	if currentVolume < vs.OverMaxVolumeMax { //Background
		vb.drawBarPart(vs.BGColor, currentVolume, vs.OverMaxVolumeMax)
	}

	//Draw the volume text
	txtStr := strconv.Itoa(currentVolume)
	myTxt := text.New(pixel.Vec{}, vb.myFont)
	myTxt.Color = vs.TextColor
	textBounds := myTxt.BoundsOf(txtStr)
	myTxt.Orig = pixel.Vec{
		X: (float64(vs.OverMaxVolumeMax*vs.PercentPixelWidth) - textBounds.W()) / 2,
		Y: float64(vs.VolumeBarHeight)/2 - textBounds.H()/2,
	}
	_, _ = myTxt.WriteString(txtStr)
	myTxt.Draw(vb.win, pixel.IM)
}

// Draw part of the volume bar (a full height rect)
func (vb *volumeBar) drawBarPart(col color.Color, v1, v2 int) {
	vb.rectImage.Clear()
	vb.rectImage.Color = col
	vb.rectImage.Push(
		pixel.V(float64(v1*vs.PercentPixelWidth), 0),
		pixel.V(float64(v2*vs.PercentPixelWidth), float64(vs.VolumeBarHeight)),
	)
	vb.rectImage.Rectangle(0)
	vb.rectImage.Draw(vb.win)
}

// After the settings have been loaded, then we can finish loading the volume bar
func (vb *volumeBar) initWindowAfterSettings() {
	globalXWO.Init(vb.win)

	//Size the volume window
	vb.win.Show()
	rect := pixel.R(
		0, 0,
		float64(vs.PercentPixelWidth*vs.OverMaxVolumeMax),
		float64(vs.VolumeBarHeight),
	)
	vb.win.SetBounds(rect)

	//Create the font (use fallback font on fail)
	newFont, err := vb.createVolumeFont()
	if err != nil {
		newFont = basicfont.Face7x13
		utils.PrintError("Error loading font: %s", err.Error())
	}
	vb.myFont = text.NewAtlas(newFont, text.ASCII)
}

// Create the volume font
func (vb *volumeBar) createVolumeFont() (font.Face, error) {
	if fontBytes, err := os.ReadFile(vs.FontPath); err != nil { //Read the font file
		return nil, err
	} else if theFont, err := opentype.Parse(fontBytes); err != nil { // Parse the font
		return nil, err
	} else if face, err := opentype.NewFace(theFont, &opentype.FaceOptions{ // Create a font face
		Size:    float64(vs.TextSize),
		DPI:     72,
		Hinting: font.HintingFull,
	}); err != nil {
		return nil, err
	} else {
		return face, nil //text.ASCII includes basic printable characters
	}
}

// Update updates the volume slider
func (vb *volumeBar) Update() {
	vb.PushCommand(vbCommandUpdateVolume)
}

// PushCommand adds a command for the volumeBar to execute
func (vb *volumeBar) PushCommand(vbc volumeBarCommand) {
	vb.commands <- vbc
}
