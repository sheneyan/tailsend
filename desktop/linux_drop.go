//go:build linux

package main

/*
#cgo !webkit2_41 pkg-config: gtk+-3.0 webkit2gtk-4.0
#cgo webkit2_41 pkg-config: gtk+-3.0 webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <string.h>

extern void goEmitDropped(char*);

static gboolean drop_installed = FALSE;
static gboolean awaiting_drop = FALSE;
static int drop_setup_tries = 0;

static void find_webview(GtkWidget *widget, gpointer data) {
	if (WEBKIT_IS_WEB_VIEW(widget)) {
		*((GtkWidget **)data) = widget;
		return;
	}
	if (GTK_IS_CONTAINER(widget)) {
		gtk_container_forall(GTK_CONTAINER(widget), find_webview, data);
	}
}

static GtkWidget *wails_webview(void) {
	GtkWidget *found = NULL;
	GList *toplevels = gtk_window_list_toplevels();
	for (GList *l = toplevels; l != NULL; l = l->next) {
		find_webview(GTK_WIDGET(l->data), &found);
		if (found != NULL) {
			break;
		}
	}
	g_list_free(toplevels);
	return found;
}

// Dest lives on the GtkWindow, not the WebView. Wails unsets the WebView dest
// (DisableWebViewDrop) so WebKit will not open the file; putting dest back on
// the WebView re-enables WebKit's own drag-motion handlers, which request
// selection data on hover and then fight gtk_drag_finish — the pointer grab
// never releases and the whole session stops taking clicks.

static gboolean on_drag_motion(GtkWidget *widget, GdkDragContext *ctx,
	gint x, gint y, guint time, gpointer user_data) {
	(void)widget; (void)x; (void)y; (void)user_data;
	gdk_drag_status(ctx, GDK_ACTION_COPY, time);
	return TRUE;
}

static gboolean on_drag_drop(GtkWidget *widget, GdkDragContext *ctx,
	gint x, gint y, guint time, gpointer user_data) {
	(void)x; (void)y; (void)user_data;
	GdkAtom target = gtk_drag_dest_find_target(widget, ctx, NULL);
	if (target == GDK_NONE) {
		awaiting_drop = FALSE;
		gtk_drag_finish(ctx, FALSE, FALSE, time);
		return TRUE;
	}
	awaiting_drop = TRUE;
	gtk_drag_get_data(widget, ctx, target, time);
	return TRUE;
}

static void on_drag_data_received(GtkWidget *widget, GdkDragContext *ctx, gint x, gint y,
	GtkSelectionData *sel, guint info, guint time, gpointer user_data) {
	(void)widget; (void)x; (void)y; (void)info; (void)user_data;
	if (!awaiting_drop) {
		return;
	}
	awaiting_drop = FALSE;

	gchar **uris = gtk_selection_data_get_uris(sel);
	if (uris == NULL) {
		const guchar *raw = gtk_selection_data_get_data(sel);
		if (raw != NULL) {
			uris = g_uri_list_extract_uris((const gchar *)raw);
		}
	}
	if (uris == NULL) {
		gtk_drag_finish(ctx, FALSE, FALSE, time);
		return;
	}
	GString *out = g_string_new("");
	for (int i = 0; uris[i] != NULL; i++) {
		gchar *path = g_filename_from_uri(uris[i], NULL, NULL);
		if (path == NULL) {
			continue;
		}
		if (out->len > 0) {
			g_string_append_c(out, '\n');
		}
		g_string_append(out, path);
		g_free(path);
	}
	g_strfreev(uris);
	if (out->len > 0) {
		goEmitDropped(out->str);
	}
	g_string_free(out, TRUE);
	gtk_drag_finish(ctx, TRUE, FALSE, time);
}

static gboolean on_decide_policy(WebKitWebView *view, WebKitPolicyDecision *decision,
	WebKitPolicyDecisionType type, gpointer user_data) {
	(void)view; (void)user_data;
	if (type != WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION &&
		type != WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) {
		return FALSE;
	}
	WebKitNavigationAction *action =
		webkit_navigation_policy_decision_get_navigation_action(WEBKIT_NAVIGATION_POLICY_DECISION(decision));
	WebKitURIRequest *req = webkit_navigation_action_get_request(action);
	const gchar *uri = webkit_uri_request_get_uri(req);
	if (uri == NULL || !g_str_has_prefix(uri, "file://")) {
		return FALSE;
	}
	// Ignore only. Emitting a drop here runs on drag-over, not mouse-up,
	// and leaves the X11 pointer grab held.
	webkit_policy_decision_ignore(decision);
	return TRUE;
}

static gboolean setup_linux_drop_idle(gpointer data) {
	(void)data;
	if (drop_installed) {
		return G_SOURCE_REMOVE;
	}
	GtkWidget *view = wails_webview();
	if (view == NULL) {
		drop_setup_tries++;
		return drop_setup_tries < 50 ? G_SOURCE_CONTINUE : G_SOURCE_REMOVE;
	}
	GtkWidget *toplevel = gtk_widget_get_toplevel(view);
	if (toplevel == NULL || !GTK_IS_WINDOW(toplevel)) {
		drop_setup_tries++;
		return drop_setup_tries < 50 ? G_SOURCE_CONTINUE : G_SOURCE_REMOVE;
	}

	gtk_drag_dest_unset(view);

	static const GtkTargetEntry targets[] = {
		{"text/uri-list", 0, 0},
		{"text/plain", 0, 1},
	};
	gtk_drag_dest_set(toplevel, GTK_DEST_DEFAULT_HIGHLIGHT, targets, 2, GDK_ACTION_COPY);
	g_signal_connect(G_OBJECT(toplevel), "drag-motion", G_CALLBACK(on_drag_motion), NULL);
	g_signal_connect(G_OBJECT(toplevel), "drag-drop", G_CALLBACK(on_drag_drop), NULL);
	g_signal_connect(G_OBJECT(toplevel), "drag-data-received", G_CALLBACK(on_drag_data_received), NULL);
	g_signal_connect(G_OBJECT(view), "decide-policy", G_CALLBACK(on_decide_policy), NULL);
	drop_installed = TRUE;
	return G_SOURCE_REMOVE;
}

void tailsendScheduleLinuxDrop(void) {
	drop_setup_tries = 0;
	g_timeout_add(100, setup_linux_drop_idle, NULL);
}
*/
import "C"

func scheduleLinuxFileDrop() {
	C.tailsendScheduleLinuxDrop()
}
