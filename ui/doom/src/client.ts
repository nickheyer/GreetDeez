import { createGreeterServiceClient, type GreeterServiceClient } from "@nickheyer/greetdeez";

let instance: GreeterServiceClient | null = null;

export function getClient(): GreeterServiceClient {
	if (!instance) {
		instance = createGreeterServiceClient();
	}
	return instance;
}
