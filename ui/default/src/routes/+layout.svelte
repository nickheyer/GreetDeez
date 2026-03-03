<script>
	import '../app.css';
	let { children } = $props();
</script>

<div class="fixed inset-0 overflow-hidden">
	<div class="aurora"></div>

	<div class="relative z-10 flex h-full w-full items-center justify-center">
		{@render children()}
	</div>
</div>
