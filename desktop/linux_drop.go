//go:build linux

package main

/*
#cgo !webkit2_41 pkg-config: gtk+-3.0 webkit2gtk-4.0
#cgo webkit2_41 pkg-config: gtk+-3.0 webkit2gtk-4.1

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <string.h>

extern void goEmitDropped(char*);

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

static void on_drag_data_received(GtkWidget *widget, GdkDragContext *ctx, gint x, gint y,
	GtkSelectionData *sel, guint info, guint time, gpointer user_data) {
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
	gchar *path = g_filename_from_uri(uri, NULL, NULL);
	if (path != NULL) {
		goEmitDropped(path);
		g_free(path);
	}
	webkit_policy_decision_ignore(decision);
	return TRUE;
}

static gboolean setup_linux_drop_idle(gpointer data) {
	GtkWidget *view = wails_webview();
	if (view == NULL) {
		return G_SOURCE_REMOVE;
	}
	static const GtkTargetEntry targets[] = {
		{"text/uri-list", 0, 0},
		{"text/plain", 0, 1},
	};
	gtk_drag_dest_set(view, GTK_DEST_DEFAULT_ALL, targets, 2, GDK_ACTION_COPY);
	g_signal_connect(G_OBJECT(view), "drag-data-received", G_CALLBACK(on_drag_data_received), NULL);
	g_signal_connect(G_OBJECT(view), "decide-policy", G_CALLBACK(on_decide_policy), NULL);
	return G_SOURCE_REMOVE;
}

void tailsendScheduleLinuxDrop(void) {
	g_idle_add(setup_linux_drop_idle, NULL);
}
*/
import "C"

func scheduleLinuxFileDrop() {
	C.tailsendScheduleLinuxDrop()
}

//export goEmitDropped
func goEmitDropped(cpaths *C.char) {
	emitDroppedPaths(splitPOSIXLines(C.GoString(cpaths)))
}
