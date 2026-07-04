// Centralized icon registry.
// Icons are rendered from lucide-react components via iconToSvg so the
// project no longer depends on lucide-static raw SVG imports.
import {
  Plus,
  X,
  Sparkles,
  Layers,
  MoreHorizontal,
  Paperclip,
  ChevronDown,
  SendHorizontal,
} from "lucide-react";
import { iconToSvg } from "./utils/icon";

export const icons = {
  plus: iconToSvg(Plus),
  x: iconToSvg(X),
  sparkles: iconToSvg(Sparkles),
  layers: iconToSvg(Layers),
  moreHorizontal: iconToSvg(MoreHorizontal),
  paperclip: iconToSvg(Paperclip),
  chevronDown: iconToSvg(ChevronDown),
  sendHorizontal: iconToSvg(SendHorizontal),
};
