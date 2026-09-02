//go:build windows

package main

/*
#cgo LDFLAGS: -lole32 -lshell32 -luuid

#define CINTERFACE
#define COBJMACROS
#include <windows.h>
#include <ole2.h>
#include <shellapi.h>
#include <string.h>
#include <stdlib.h>

extern void goWinDropped(char*);

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

static void emit_hdrop(HDROP drop) {
	UINT n = DragQueryFileW(drop, 0xFFFFFFFF, NULL, 0);
	size_t cap = 4096;
	char *out = (char *)malloc(cap);
	if (out == NULL) {
		return;
	}
	size_t len = 0;
	out[0] = 0;
	for (UINT i = 0; i < n; i++) {
		wchar_t w[32768];
		if (DragQueryFileW(drop, i, w, 32768) == 0) {
			continue;
		}
		char u[65536];
		int ul = WideCharToMultiByte(CP_UTF8, 0, w, -1, u, (int)sizeof(u), NULL, NULL);
		if (ul <= 1) {
			continue;
		}
		size_t add = (size_t)ul - 1;
		if (len + add + 2 > cap) {
			cap = (len + add + 2) * 2;
			char *nbuf = (char *)realloc(out, cap);
			if (nbuf == NULL) {
				free(out);
				return;
			}
			out = nbuf;
		}
		if (len > 0) {
			out[len++] = '\n';
		}
		memcpy(out + len, u, add);
		len += add;
		out[len] = 0;
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
	HDROP drop = (HDROP)GlobalLock(stg.hGlobal);
	if (drop != NULL) {
		emit_hdrop(drop);
		GlobalUnlock(stg.hGlobal);
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
static WNDPROC g_old_proc = NULL;
static const UINT kInstallDropMsg = WM_APP + 0x54;

static void install_on_hwnd(HWND hwnd) {
	RevokeDragDrop(hwnd);
	RegisterDragDrop(hwnd, &g_drop.idt);
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
		if (FAILED(OleInitialize(NULL))) {
			return;
		}
		g_ole_inited = TRUE;
	}
	install_on_hwnd(hwnd);
	EnumChildWindows(hwnd, enum_child, 0);
}

static VOID CALLBACK install_drop_timer(HWND hwnd, UINT msg, UINT_PTR id, DWORD now) {
	(void)msg; (void)now;
	install_on_tree(hwnd);
	g_drop_tries++;
	if (g_drop_tries >= 15) {
		KillTimer(hwnd, id);
	}
}

static LRESULT CALLBACK drop_subclass_proc(HWND hwnd, UINT msg, WPARAM w, LPARAM l) {
	if (msg == kInstallDropMsg) {
		g_drop_tries = 0;
		install_on_tree(hwnd);
		SetTimer(hwnd, 0x5444, 200, install_drop_timer);
		return 0;
	}
	if (g_old_proc != NULL) {
		return CallWindowProcW(g_old_proc, hwnd, msg, w, l);
	}
	return DefWindowProcW(hwnd, msg, w, l);
}

void tailsendScheduleWindowsDrop(UINT_PTR hwndBits) {
	HWND hwnd = (HWND)hwndBits;
	if (hwnd == NULL) {
		return;
	}
	if (g_old_proc == NULL) {
		g_old_proc = (WNDPROC)SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)drop_subclass_proc);
	}
	PostMessageW(hwnd, kInstallDropMsg, 0, 0);
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
			hwnd := findTailsendHWND()
			if hwnd != 0 {
				C.tailsendScheduleWindowsDrop(C.UINT_PTR(hwnd))
				return
			}
		}
	}()
}
