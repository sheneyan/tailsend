export function EventsOn(eventName, callback) {
  if (window.runtime && window.runtime.EventsOn) {
    return window.runtime.EventsOn(eventName, callback);
  }
  return () => {};
}

export function EventsOff(eventName, ...additionalEventNames) {
  if (window.runtime && window.runtime.EventsOff) {
    return window.runtime.EventsOff(eventName, ...additionalEventNames);
  }
}

export function OnFileDrop(callback, useDropTarget) {
  if (window.runtime && window.runtime.OnFileDrop) {
    window.runtime.OnFileDrop(callback, useDropTarget);
  }
}

export function OnFileDropOff() {
  window.runtime?.OnFileDropOff?.();
}

export function Quit() {
  window.runtime?.Quit?.();
}
