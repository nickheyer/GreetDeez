/// <reference types="vite/client" />

declare module "troika-three-text" {
	import { Mesh, Material } from "three";

	export class Text extends Mesh {
		text: string;
		font: string | null;
		fontSize: number;
		color: number | string;
		anchorX: "left" | "center" | "right";
		anchorY: "top" | "middle" | "bottom";
		letterSpacing: number;
		maxWidth: number;
		material: Material;
		sync(callback?: () => void): void;
	}
}
