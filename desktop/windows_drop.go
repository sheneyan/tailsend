//go:build windows

package main

/*
#cgo LDFLAGS: -lole32 -lshell32 -luuid -lcomctl32

#define CINTERFACE
#define COBJMACROS
#include <windows.h>
#include <ole2.h>
#include <shellapi.h>
#include <commctrl.h>
#include <string.h>
#include <stdlib.h>

extern void goWinDropped(char*);

/* MinGW shellapi.h often omits DROPFILES unless extra SDK headers are present. */
typedef struct TailsendDropFiles {
	DWORD pFiles;
	POINT pt;
	BOOL fNC;
	BOOL fWide;
} TailsendDropFiles;

typedef struct TailsendDropTarget {
	IDropTarget idt;
	LONG ref;
} TailsendDropTarget;

static HRESULT STDMETHODCALLTYPE dtQueryInterface(IDropTarget *this, REFIID riid, void **ppv) {
	if (IsEqualIID(riid, &IID_IUnknown) || IsEqualIID(riid, &IID_IDropTarget)) {
		*ppv = this;
		this->lpVtbl->AddRef(this);
		return S_OK;
	}
	*ppv = NULL;
	return E_NOINTERFACE;
}

static ULONG STDMETHODCALLTYPE dtAddRef(IDropTarget *this) {
	TailsendDropTarget *dt = (TailsendDropTarget *)this;
	return InterlockedIncrement(&dt->ref);
}

static ULONG STDMETHODCALLTYPE dtRelease(IDropTarget *this) {
	TailsendDropTarget *dt = (TailsendDropTarget *)this;
	LONG n = InterlockedDecrement(&dt->ref);
	if (n == 0) {
		return 0;
	}
	return (ULONG)n;
}

static BOOL has_hdrop(IDataObject *obj) {
	FORMATETC fe = { CF_HDROP, NULL, DVASPECT_CONTENT, -1, TYMED_HGLOBAL };
	return obj->lpVtbl->QueryGetData(obj, &fe) == S_OK;
}

static HRESULT STDMETHODCALLTYPE dtDragEnter(IDropTarget *this, IDataObject *obj,
	DWORD keys, POINTL pt, DWORD *effect) {
	(void)this; (void)keys; (void)pt;
	if (has_hdrop(obj)) {
		*effect &= DROPEFFECT_COPY;
		return S_OK;
	}
	*effect = DROPEFFECT_NONE;
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE dtDragOver(IDropTarget *this, DWORD keys, POINTL pt, DWORD *effect) {
	(void)this; (void)keys; (void)pt;
	*effect &= DROPEFFECT_COPY;
	return S_OK;
}

static HRESULT STDMETHODCALLTYPE dtDragLeave(IDropTarget *this) {
	(void)this;
	return S_OK;
}

static int append_utf8(char **out, size_t *len, size_t *cap, const char *u, size_t add) {
	if (*len + add + 2 > *cap) {
		size_t ncap = (*len + add + 2) * 2;
		char *nbuf = (char *)realloc(*out, ncap);
		if (nbuf == NULL) {
			return 0;
		}
		*out = nbuf;
		*cap = ncap;
	}
	if (*len > 0) {
		(*out)[(*len)++] = '\n';
	}
	memcpy(*out + *len, u, add);
	*len += add;
	(*out)[*len] = 0;
	return 1;
}

static void append_wide(char **out, size_t *len, size_t *cap, const wchar_t *w) {
	char u[65536];
	int ul = WideCharToMultiByte(CP_UTF8, 0, w, -1, u, (int)sizeof(u), NULL, NULL);
	if (ul > 1) {
		append_utf8(out, len, cap, u, (size_t)ul - 1);
	}
}

static void emit_from_dropfiles(TailsendDropFiles *df, char **out, size_t *len, size_t *cap) {
	if (df == NULL || df->pFiles == 0) {
		return;
	}
	if (df->fWide) {
		const wchar_t *w = (const wchar_t *)((const char *)df + df->pFiles);
		while (*w) {
			append_wide(out, len, cap, w);
			w += wcslen(w) + 1;
		}
		return;
	}
	const char *a = (const char *)df + df->pFiles;
	while (*a) {
		size_t add = strlen(a);
		append_utf8(out, len, cap, a, add);
		a += add + 1;
	}
}

static void emit_hdrop(HDROP drop) {
	if (drop == NULL) {
		return;
	}
	size_t cap = 4096;
	char *out = (char *)malloc(cap);
	if (out == NULL) {
		return;
	}
	size_t len = 0;
	out[0] = 0;
	UINT n = DragQueryFileW(drop, 0xFFFFFFFF, NULL, 0);
	for (UINT i = 0; i < n; i++) {
		wchar_t w[32768];
		if (DragQueryFileW(drop, i, w, 32768) == 0) {
			continue;
		}
		append_wide(&out, &len, &cap, w);
	}
	if (len == 0) {
		TailsendDropFiles *df = (TailsendDropFiles *)GlobalLock(drop);
		if (df != NULL) {
			emit_from_dropfiles(df, &out, &len, &cap);
			GlobalUnlock(drop);
		}
	}
	if (len > 0) {
		goWinDropped(out);
	}
	free(out);
}

static HRESULT STDMETHODCALLTYPE dtDrop(IDropTarget *this, IDataObject *obj,
	DWORD keys, POINTL pt, DWORD *effect) {
	(void)this; (void)keys; (void)pt;
	FORMATETC fe = { CF_HDROP, NULL, DVASPECT_CONTENT, -1, TYMED_HGLOBAL };
	STGMEDIUM stg;
	if (FAILED(obj->lpVtbl->GetData(obj, &fe, &stg))) {
		*effect = DROPEFFECT_NONE;
		return S_OK;
	}
	// HDROP is the HGLOBAL handle itself. GlobalLock returns a DROPFILES*
	// pointer; DragQueryFileW expects the handle, not the locked pointer.
	if (stg.tymed == TYMED_HGLOBAL && stg.hGlobal != NULL) {
		emit_hdrop((HDROP)stg.hGlobal);
	}
	ReleaseStgMedium(&stg);
	*effect = DROPEFFECT_COPY;
	return S_OK;
}

static IDropTargetVtbl g_vtbl = {
	dtQueryInterface,
	dtAddRef,
	dtRelease,
	dtDragEnter,
	dtDragOver,
	dtDragLeave,
	dtDrop,
};

static TailsendDropTarget g_drop = { { &g_vtbl }, 1 };
static BOOL g_ole_inited = FALSE;
static int g_drop_tries = 0;
static const UINT kInstallDropMsg = WM_APP + 0x54;
static const UINT_PTR kSubclassID = 0x5444;

static HWND g_drop_seen[256];
static int g_drop_nseen = 0;

static BOOL drop_already(HWND hwnd) {
	for (int i = 0; i < g_drop_nseen; i++) {
		if (g_drop_seen[i] == hwnd) {
			return TRUE;
		}
	}
	return FALSE;
}

static void install_on_hwnd(HWND hwnd) {
	if (hwnd == NULL || drop_already(hwnd)) {
		return;
	}
	DragAcceptFiles(hwnd, TRUE);
	HRESULT hr = RegisterDragDrop(hwnd, &g_drop.idt);
	if (hr == DRAGDROP_E_ALREADYREGISTERED) {
		RevokeDragDrop(hwnd);
		hr = RegisterDragDrop(hwnd, &g_drop.idt);
	}
	if (SUCCEEDED(hr) && g_drop_nseen < 256) {
		g_drop_seen[g_drop_nseen++] = hwnd;
	}
}

static BOOL CALLBACK enum_child(HWND child, LPARAM lp) {
	(void)lp;
	install_on_hwnd(child);
	return TRUE;
}

static void install_on_tree(HWND hwnd) {
	if (hwnd == NULL) {
		return;
	}
	if (!g_ole_inited) {
		HRESULT ohr = OleInitialize(NULL);
		if (SUCCEEDED(ohr) || ohr == RPC_E_CHANGED_MODE) {
			g_ole_inited = TRUE;
		} else {
			return;
		}
	}
	install_on_hwnd(hwnd);
	EnumChildWindows(hwnd, enum_child, 0);
}

static VOID CALLBACK install_drop_timer(HWND hwnd, UINT msg, UINT_PTR id, DWORD now) {
	(void)msg; (void)now;
	install_on_tree(hwnd);
	g_drop_tries++;
	if (g_drop_tries >= 20) {
		KillTimer(hwnd, id);
	}
}

static LRESULT CALLBACK drop_subclass(HWND hwnd, UINT msg, WPARAM w, LPARAM l,
	UINT_PTR id, DWORD_PTR ref) {
	(void)ref;
	if (msg == kInstallDropMsg) {
		g_drop_tries = 0;
		install_on_tree(hwnd);
		SetTimer(hwnd, 0x5444, 250, install_drop_timer);
		return 0;
	}
	if (msg == WM_DROPFILES) {
		emit_hdrop((HDROP)w);
		DragFinish((HDROP)w);
		return 0;
	}
	if (msg == WM_NCDESTROY) {
		RemoveWindowSubclass(hwnd, drop_subclass, id);
	}
	return DefSubclassProc(hwnd, msg, w, l);
}

static BOOL CALLBACK find_wails_frame(HWND hwnd, LPARAM lp) {
	DWORD pid = 0;
	GetWindowThreadProcessId(hwnd, &pid);
	if (pid != GetCurrentProcessId()) {
		return TRUE;
	}
	wchar_t cls[256];
	if (GetClassNameW(hwnd, cls, 256) == 0) {
		return TRUE;
	}
	if (wcscmp(cls, L"wailsWindow") == 0) {
		*(HWND *)lp = hwnd;
		return FALSE;
	}
	return TRUE;
}

void tailsendScheduleWindowsDrop(void) {
	HWND frame = NULL;
	EnumWindows(find_wails_frame, (LPARAM)&frame);
	if (frame == NULL) {
		return;
	}
	SetWindowSubclass(frame, drop_subclass, kSubclassID, 0);
	PostMessageW(frame, kInstallDropMsg, 0, 0);
}
*/
import "C"

import (
	"time"
)

func scheduleWindowsFileDrop() {
	go func() {
		for i := 0; i < 50; i++ {
			time.Sleep(100 * time.Millisecond)
			C.tailsendScheduleWindowsDrop()
		}
	}()
}
