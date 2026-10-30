<!--
	Editor for free-form device settings (key, type, value). ReefCare stores them as typed
	and never interprets them. Field errors arrive keyed as `settings.<key>`.
-->
<script lang="ts">
	import type { SettingRow } from '#lib/equipment.js';
	import { m } from '#lib/paraglide/messages.js';

	type Props = { rows: SettingRow[]; errors: Record<string, string> };
	let { rows = $bindable(), errors }: Props = $props();
	const uid = $props.id();

	function add() {
		rows = [...rows, { key: '', kind: 'text', value: '' }];
	}
	function remove(i: number) {
		rows = rows.filter((_, j) => j !== i);
	}
</script>

<fieldset class="settings">
	<legend>{m.eq_settings_label()}</legend>
	<p class="hint">{m.eq_settings_hint()}</p>
	{#each rows as row, i (i)}
		{@const err = errors[`settings.${row.key.trim()}`]}
		<div class="row">
			<label>
				<span>{m.eq_setting_key()}</span>
				<input
					name="key"
					bind:value={row.key}
					maxlength="50"
					aria-invalid={err ? 'true' : undefined}
					aria-describedby={err ? `${uid}-${i}-error` : undefined}
				/>
			</label>
			<label>
				<span>{m.eq_setting_kind()}</span>
				<select bind:value={row.kind}>
					<option value="text">{m.eq_kind_text()}</option>
					<option value="number">{m.eq_kind_number()}</option>
					<option value="boolean">{m.eq_kind_boolean()}</option>
				</select>
			</label>
			{#if row.kind === 'boolean'}
				<label class="check">
					<input
						type="checkbox"
						checked={row.value === 'true'}
						onchange={(e) => (row.value = String(e.currentTarget.checked))}
					/>
					<span>{m.eq_setting_value()}</span>
				</label>
			{:else}
				<label>
					<span>{m.eq_setting_value()}</span>
					<input
						name="value"
						bind:value={row.value}
						maxlength="200"
						inputmode={row.kind === 'number' ? 'decimal' : undefined}
					/>
				</label>
			{/if}
			<button type="button" class="secondary" onclick={() => remove(i)}
				>{m.eq_setting_remove()}</button
			>
		</div>
		{#if err}
			<p id="{uid}-{i}-error" class="field-error"><span aria-hidden="true">⚠</span> {err}</p>
		{/if}
	{/each}
	{#if errors.settings}
		<p class="field-error"><span aria-hidden="true">⚠</span> {errors.settings}</p>
	{/if}
	<button type="button" class="secondary" onclick={add}>{m.eq_setting_add()}</button>
</fieldset>

<style>
	.settings {
		border: 1px solid var(--border, #ccd);
		border-radius: 0.5rem;
		padding: 0.75rem 1rem;
	}
	.row {
		display: flex;
		flex-wrap: wrap;
		gap: 0.5rem;
		align-items: end;
		margin-bottom: 0.5rem;
	}
	.row label {
		display: flex;
		flex-direction: column;
		font-weight: normal;
		gap: 0.15rem;
	}
	.row .check {
		flex-direction: row;
		align-items: center;
	}
</style>
