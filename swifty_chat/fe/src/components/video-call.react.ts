import React from "react";
import { createComponent } from "@lit/react";
import { VideoCall } from "./video-call";

/**
 * The React wrapper exposes the underlying Lit element instance via `ref`,
 * so callers can invoke `ref.current?.show()` and
 * `ref.current?.handleSignal(avData)` directly.
 */
export const VideoCallComponent = createComponent({
  tagName: "video-call",
  elementClass: VideoCall,
  react: React,
});
