//X windows operations not available in pixelGL

package plugin_volume

/*
#cgo pkg-config: x11 xext
#define GLFW_EXPOSE_NATIVE_X11
#include "./glfw/glfw3.h"
#include "./glfw/glfw3native.h"
#include "./X11/Xlib.h"
#include "./X11/Xatom.h"
#include "./X11/extensions/shape.h"
*/
import "C"
import (
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/gopxl/pixel/pixelgl"
	"reflect"
	"script_server/utils"
	"unsafe"
)

type xWinOps struct {
	xWin C.Window //If this is nil then member functions will not run
	dpy  *C.Display
}

var globalXWO = &xWinOps{}

func (xwo *xWinOps) Init(pixelGlWin *pixelgl.Window) {
	//Check to see if we want this functionality
	const FuncErr = "Unavailable functionality: Hide from taskbar, make non-focusable, mouse pass-through, keep on top"
	if vs.RunExtraXWinCode < 1 {
		utils.PrintError("RunExtraXWinCode is turned off")
		utils.PrintError(FuncErr)
		return
	}

	//Run unsafe lookups to get needed x11 data
	func() {
		defer func() {
			if r := recover(); r != nil {
				utils.PrintError("Failed to get x11 data: %v", r)
				utils.PrintError(FuncErr)
				xwo.dpy = nil
			}
		}()
		glfwWin := (*glfw.Window)(reflect.ValueOf(pixelGlWin).Elem().FieldByName("window").UnsafePointer())
		xwo.xWin = C.glfwGetX11Window((*C.GLFWwindow)(glfwWin.Handle()))
		xwo.dpy = C.glfwGetX11Display()
	}()

	//Run x11 functions for the first time to make sure nothing goes wrong
	func() {
		defer func() {
			if r := recover(); r != nil {
				utils.PrintError("Failed to call x11 functions: %v", r)
				utils.PrintError(FuncErr)
				xwo.dpy = nil
			}
		}()
		xwo.HideFromTaskbar()
		xwo.MakeNonFocusable()
		xwo.SetMousePassThrough()
		xwo.SetOnTop()
	}()
}

func (xwo *xWinOps) makeAtom(str string) C.Atom {
	return C.XInternAtom(xwo.dpy, C.CString(str), C.False)
}
func (xwo *xWinOps) flush() { C.XFlush(xwo.dpy) }

func (xwo *xWinOps) SetMousePassThrough() {
	if xwo.dpy == nil {
		return
	}

	var eventBase, errorBase C.int
	if C.XShapeQueryExtension(xwo.dpy, &eventBase, &errorBase) != 0 {
		region := C.XCreateRegion() //Empty region
		C.XShapeCombineRegion(xwo.dpy, xwo.xWin, C.ShapeInput, 0, 0, region, C.ShapeSet)
		C.XDestroyRegion(region)
	}
}

func (xwo *xWinOps) HideFromTaskbar() {
	if xwo.dpy == nil {
		return
	}

	atom := xwo.makeAtom("_NET_WM_STATE_SKIP_TASKBAR")
	C.XChangeProperty(
		xwo.dpy, xwo.xWin, xwo.makeAtom("_NET_WM_STATE"),
		C.XA_ATOM, 32, C.PropModeAppend, (*C.uchar)(unsafe.Pointer(&atom)), 1,
	)
	xwo.flush()
}

func (xwo *xWinOps) MakeNonFocusable() {
	if xwo.dpy == nil {
		return
	}

	hints := C.XWMHints{
		flags: C.InputHint,
		input: C.int(C.False),
	}
	C.XSetWMHints(xwo.dpy, xwo.xWin, &hints)
	xwo.flush()
}

func (xwo *xWinOps) SetOnTop() {
	if xwo.dpy == nil {
		return
	}

	const netWmStateAdd = 1 //_NET_WM_STATE_ADD
	ev := C.XClientMessageEvent{
		_type:        C.ClientMessage,
		window:       xwo.xWin,
		message_type: xwo.makeAtom("_NET_WM_STATE"),
		format:       32,
		data: *(*[40]byte)(unsafe.Pointer(&[5]C.long{
			netWmStateAdd, C.long(xwo.makeAtom("_NET_WM_STATE_ABOVE")), 0, 1, 0,
		})),
	}
	C.XSendEvent(
		xwo.dpy,
		C.XRootWindow(xwo.dpy, C.XDefaultScreen(xwo.dpy)),
		C.False,
		C.SubstructureRedirectMask|C.SubstructureNotifyMask,
		(*C.XEvent)(unsafe.Pointer(&ev)),
	)
	xwo.flush()
}
