import { Text } from "troika-three-text";
import fontUrl from "../assets/fonts/JetBrainsMono-Regular.ttf?url";

export interface TextOptions {
	text?: string;
	fontSize?: number;
	color?: number;
	anchorX?: "left" | "center" | "right";
	anchorY?: "top" | "middle" | "bottom";
	letterSpacing?: number;
	maxWidth?: number;
}

export function createText(opts: TextOptions = {}): Text {
	const t = new Text();
	t.font = fontUrl;
	t.text = opts.text ?? "";
	t.fontSize = opts.fontSize ?? 0.08;
	t.color = opts.color ?? 0xe0e0ff;
	t.anchorX = opts.anchorX ?? "left";
	t.anchorY = opts.anchorY ?? "middle";
	t.letterSpacing = opts.letterSpacing ?? 0.02;
	if (opts.maxWidth !== undefined) t.maxWidth = opts.maxWidth;
	t.sync();
	return t;
}
