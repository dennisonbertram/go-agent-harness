//go:build darwin && cgo

package nativegui

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreGraphics
#include <ApplicationServices/ApplicationServices.h>
#include <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>

static AXUIElementRef gocodeAXApplication(pid_t pid) {
    return AXUIElementCreateApplication(pid);
}

static void gocodeCFRelease(const void *value) {
    if (value != NULL) CFRelease(value);
}

static CFTypeRef gocodeAXCopyAttribute(AXUIElementRef element, const char *name) {
    CFStringRef attribute = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (attribute == NULL) return NULL;
    CFTypeRef value = NULL;
    AXError err = AXUIElementCopyAttributeValue(element, attribute, &value);
    CFRelease(attribute);
    return err == kAXErrorSuccess ? value : NULL;
}

static char *gocodeAXCopyString(AXUIElementRef element, const char *name) {
    CFTypeRef value = gocodeAXCopyAttribute(element, name);
    if (value == NULL) return NULL;
    CFStringRef text = NULL;
    if (CFGetTypeID(value) == CFStringGetTypeID()) {
        text = (CFStringRef)value;
        CFRetain(text);
    } else {
        text = CFCopyDescription(value);
    }
    CFRelease(value);
    if (text == NULL) return NULL;
    CFIndex size = CFStringGetMaximumSizeForEncoding(CFStringGetLength(text), kCFStringEncodingUTF8) + 1;
    char *result = (char *)malloc((size_t)size);
    if (result == NULL || !CFStringGetCString(text, result, size, kCFStringEncodingUTF8)) {
        free(result);
        result = NULL;
    }
    CFRelease(text);
    return result;
}

static CFIndex gocodeAXChildCount(AXUIElementRef element) {
    CFTypeRef value = gocodeAXCopyAttribute(element, "AXChildren");
    if (value == NULL || CFGetTypeID(value) != CFArrayGetTypeID()) {
        if (value != NULL) CFRelease(value);
        return 0;
    }
    CFIndex count = CFArrayGetCount((CFArrayRef)value);
    CFRelease(value);
    return count;
}

static AXUIElementRef gocodeAXCopyChild(AXUIElementRef element, CFIndex index) {
    CFTypeRef value = gocodeAXCopyAttribute(element, "AXChildren");
    if (value == NULL || CFGetTypeID(value) != CFArrayGetTypeID()) {
        if (value != NULL) CFRelease(value);
        return NULL;
    }
    CFArrayRef children = (CFArrayRef)value;
    if (index < 0 || index >= CFArrayGetCount(children)) {
        CFRelease(value);
        return NULL;
    }
    AXUIElementRef child = (AXUIElementRef)CFArrayGetValueAtIndex(children, index);
    CFRetain(child);
    CFRelease(value);
    return child;
}

static int gocodeAXSetString(AXUIElementRef element, const char *name, const char *text) {
    CFStringRef attribute = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    CFStringRef value = CFStringCreateWithCString(NULL, text, kCFStringEncodingUTF8);
    if (attribute == NULL || value == NULL) {
        if (attribute != NULL) CFRelease(attribute);
        if (value != NULL) CFRelease(value);
        return 0;
    }
    AXError err = AXUIElementSetAttributeValue(element, attribute, value);
    CFRelease(attribute);
    CFRelease(value);
    return err == kAXErrorSuccess ? 1 : 0;
}

static int gocodeAXSetBool(AXUIElementRef element, const char *name, int enabled) {
    CFStringRef attribute = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (attribute == NULL) return 0;
    AXError err = AXUIElementSetAttributeValue(element, attribute, enabled ? kCFBooleanTrue : kCFBooleanFalse);
    CFRelease(attribute);
    return err == kAXErrorSuccess ? 1 : 0;
}

static int gocodeAXPerform(AXUIElementRef element, const char *name) {
    CFStringRef action = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (action == NULL) return 0;
    AXError err = AXUIElementPerformAction(element, action);
    CFRelease(action);
    return err == kAXErrorSuccess ? 1 : 0;
}

static uint32_t gocodeLargestWindow(pid_t pid) {
    CFArrayRef windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
    if (windows == NULL) return 0;
    uint32_t best = 0;
    double bestArea = 0;
    CFIndex count = CFArrayGetCount(windows);
    for (CFIndex i = 0; i < count; i++) {
        CFDictionaryRef window = (CFDictionaryRef)CFArrayGetValueAtIndex(windows, i);
        CFNumberRef owner = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowOwnerPID);
        int ownerPID = 0;
        if (owner == NULL || !CFNumberGetValue(owner, kCFNumberIntType, &ownerPID) || ownerPID != pid) continue;
        CFNumberRef layerValue = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowLayer);
        int layer = -1;
        if (layerValue == NULL || !CFNumberGetValue(layerValue, kCFNumberIntType, &layer) || layer != 0) continue;
        CGRect bounds = CGRectZero;
        CFDictionaryRef boundsValue = (CFDictionaryRef)CFDictionaryGetValue(window, kCGWindowBounds);
        if (boundsValue == NULL || !CGRectMakeWithDictionaryRepresentation(boundsValue, &bounds)) continue;
        double area = bounds.size.width * bounds.size.height;
        if (area > bestArea) {
            CFNumberRef number = (CFNumberRef)CFDictionaryGetValue(window, kCGWindowNumber);
            int windowID = 0;
            if (number != NULL && CFNumberGetValue(number, kCFNumberIntType, &windowID)) {
                best = (uint32_t)windowID;
                bestArea = area;
            }
        }
    }
    CFRelease(windows);
    return best;
}
*/
import "C"

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type DarwinCorePlatform struct{}

type axNode struct {
	Role        string   `json:"role,omitempty"`
	Title       string   `json:"title,omitempty"`
	Description string   `json:"description,omitempty"`
	Placeholder string   `json:"placeholder,omitempty"`
	Value       string   `json:"value,omitempty"`
	Children    []axNode `json:"children,omitempty"`
	ref         unsafe.Pointer
}

func (DarwinCorePlatform) SubmitPrompt(ctx context.Context, pid int, prompt string) error {
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		root := axApplication(pid)
		if root == nil {
			return fmt.Errorf("create AX application for attested PID %d", pid)
		}
		axSetBool(root, "AXFrontmost", true)
		remaining := 1200
		tree := readAXTree(root, 0, &remaining)
		composer := findAXNode(&tree, func(node *axNode) bool {
			return node.Role == "AXTextArea" || node.Role == "AXTextField"
		})
		if composer != nil {
			value := C.CString(prompt)
			attribute := C.CString("AXValue")
			set := C.gocodeAXSetString(C.AXUIElementRef(composer.ref), attribute, value) == 1
			C.free(unsafe.Pointer(value))
			C.free(unsafe.Pointer(attribute))
			if set {
				axSetBool(composer.ref, "AXFocused", true)
				button := findAXNode(&tree, func(node *axNode) bool {
					label := strings.ToLower(node.Title + " " + node.Description)
					return node.Role == "AXButton" && strings.Contains(label, "send message")
				})
				if button != nil && axPerform(button.ref, "AXPress") {
					releaseAXTree(&tree)
					return nil
				}
			}
		}
		releaseAXTree(&tree)
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("attested app PID %d never exposed a writable composer and Send message control", pid)
}

func (DarwinCorePlatform) AccessibilitySnapshot(ctx context.Context, pid int) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	root := axApplication(pid)
	if root == nil {
		return nil, fmt.Errorf("create AX application for attested PID %d", pid)
	}
	remaining := 2000
	tree := readAXTree(root, 0, &remaining)
	data, err := json.Marshal(tree)
	releaseAXTree(&tree)
	if err != nil {
		return nil, fmt.Errorf("marshal AX tree: %w", err)
	}
	return data, nil
}

func axApplication(pid int) unsafe.Pointer {
	return unsafe.Pointer(C.gocodeAXApplication(C.pid_t(pid)))
}

func (DarwinCorePlatform) CaptureScreenshot(ctx context.Context, pid int, path string) error {
	windowID := uint32(C.gocodeLargestWindow(C.pid_t(pid)))
	if windowID == 0 {
		return fmt.Errorf("attested app PID %d has no on-screen layer-zero window", pid)
	}
	command := exec.CommandContext(ctx, "/usr/sbin/screencapture", "-x", "-o", "-l"+strconv.FormatUint(uint64(windowID), 10), path)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("capture attested window %d: %w: %s", windowID, err, strings.TrimSpace(string(output)))
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() == 0 {
		return fmt.Errorf("screencapture did not produce a nonempty regular PNG")
	}
	return nil
}

func readAXTree(ref unsafe.Pointer, depth int, remaining *int) axNode {
	node := axNode{ref: ref}
	*remaining--
	node.Role = axString(ref, "AXRole")
	node.Title = axString(ref, "AXTitle")
	node.Description = axString(ref, "AXDescription")
	node.Placeholder = axString(ref, "AXPlaceholderValue")
	node.Value = axString(ref, "AXValue")
	if depth >= 30 || *remaining <= 0 {
		return node
	}
	count := int(C.gocodeAXChildCount(C.AXUIElementRef(ref)))
	if count > *remaining {
		count = *remaining
	}
	for i := 0; i < count; i++ {
		child := unsafe.Pointer(C.gocodeAXCopyChild(C.AXUIElementRef(ref), C.CFIndex(i)))
		if child == nil {
			continue
		}
		node.Children = append(node.Children, readAXTree(child, depth+1, remaining))
		if *remaining <= 0 {
			break
		}
	}
	return node
}

func releaseAXTree(node *axNode) {
	for i := range node.Children {
		releaseAXTree(&node.Children[i])
	}
	if node.ref != nil {
		C.gocodeCFRelease(node.ref)
		node.ref = nil
	}
}

func findAXNode(node *axNode, match func(*axNode) bool) *axNode {
	if match(node) {
		return node
	}
	for i := range node.Children {
		if found := findAXNode(&node.Children[i], match); found != nil {
			return found
		}
	}
	return nil
}

func axString(ref unsafe.Pointer, attribute string) string {
	name := C.CString(attribute)
	value := C.gocodeAXCopyString(C.AXUIElementRef(ref), name)
	C.free(unsafe.Pointer(name))
	if value == nil {
		return ""
	}
	defer C.free(unsafe.Pointer(value))
	return C.GoString(value)
}

func axSetBool(ref unsafe.Pointer, attribute string, value bool) bool {
	name := C.CString(attribute)
	defer C.free(unsafe.Pointer(name))
	enabled := C.int(0)
	if value {
		enabled = 1
	}
	return C.gocodeAXSetBool(C.AXUIElementRef(ref), name, enabled) == 1
}

func axPerform(ref unsafe.Pointer, action string) bool {
	name := C.CString(action)
	defer C.free(unsafe.Pointer(name))
	return C.gocodeAXPerform(C.AXUIElementRef(ref), name) == 1
}
