<script>
	import '../app.css';
	let { children } = $props();
</script>

<div class="fixed inset-0 overflow-hidden">
	<div class="aurora aurora-1"></div>
	<div class="aurora aurora-2"></div>
	<div class="aurora aurora-3"></div>

	<div class="relative z-10 flex h-full w-full items-center justify-center">
		{@render children()}
	</div>
</div>
