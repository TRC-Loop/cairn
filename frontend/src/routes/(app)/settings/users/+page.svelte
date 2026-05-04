<!-- SPDX-License-Identifier: AGPL-3.0-or-later -->
<script lang="ts">
	import { onMount } from 'svelte';
	import { _ } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Select from '$lib/components/ui/select';
	import * as Table from '$lib/components/ui/table';
	import {
		Dialog,
		DialogContent,
		DialogHeader,
		DialogTitle,
		DialogDescription,
		DialogFooter
	} from '$lib/components/ui/dialog';
	import {
		DropdownMenu,
		DropdownMenuTrigger,
		DropdownMenuContent,
		DropdownMenuItem
	} from '$lib/components/ui/dropdown-menu';
	import { IconDotsVertical } from '@tabler/icons-svelte';
	import {
		listUsers,
		createUser,
		updateUser,
		changePassword,
		deleteUser,
		type UserRecord,
		type Role
	} from '$lib/api/users';
	import { auth } from '$lib/stores/auth';
	import { toastError, toastSuccess } from '$lib/toast';
	import { adminReset2FA } from '$lib/api/twofa';

	const me = $derived($auth.user);

	let users = $state<UserRecord[]>([]);
	let loading = $state(true);

	let createOpen = $state(false);
	let cUsername = $state('');
	let cEmail = $state('');
	let cDisplay = $state('');
	let cPassword = $state('');
	let cRole = $state<string>('viewer');
	let cSaving = $state(false);

	let editTarget = $state<UserRecord | null>(null);
	let editOpen = $state(false);
	let eEmail = $state('');
	let eDisplay = $state('');
	let eRole = $state<string>('viewer');
	let eSaving = $state(false);

	let pwTarget = $state<UserRecord | null>(null);
	let pwOpen = $state(false);
	let pwNew = $state('');
	let pwForce = $state(false);
	let pwSaving = $state(false);

	let delTarget = $state<UserRecord | null>(null);
	let delOpen = $state(false);
	let delSaving = $state(false);

	let resetTarget = $state<UserRecord | null>(null);
	let resetOpen = $state(false);
	let resetSaving = $state(false);

	const adminCount = $derived(users.filter((u) => u.role === 'admin').length);

	async function refresh() {
		loading = true;
		try {
			users = await listUsers();
		} catch (err) {
			toastError(err);
		} finally {
			loading = false;
		}
	}

	onMount(refresh);

	function openCreate() {
		cUsername = '';
		cEmail = '';
		cDisplay = '';
		cPassword = '';
		cRole = 'viewer';
		createOpen = true;
	}

	async function submitCreate(e: Event) {
		e.preventDefault();
		cSaving = true;
		try {
			await createUser({
				username: cUsername,
				email: cEmail,
				display_name: cDisplay,
				password: cPassword,
				role: cRole as Role
			});
			toastSuccess($_('settings.users.created'));
			createOpen = false;
			await refresh();
		} catch (err) {
			toastError(err);
		} finally {
			cSaving = false;
		}
	}

	function openEdit(u: UserRecord) {
		editTarget = u;
		eEmail = u.email;
		eDisplay = u.display_name;
		eRole = u.role;
		editOpen = true;
	}

	async function submitEdit(e: Event) {
		e.preventDefault();
		if (!editTarget) return;
		eSaving = true;
		try {
			const isSelf = me?.id === editTarget.id;
			await updateUser(editTarget.id, {
				email: eEmail,
				display_name: eDisplay,
				role: isSelf ? undefined : (eRole as Role)
			});
			toastSuccess($_('settings.users.updated'));
			editOpen = false;
			await refresh();
		} catch (err) {
			toastError(err);
		} finally {
			eSaving = false;
		}
	}

	function openPassword(u: UserRecord) {
		pwTarget = u;
		pwNew = '';
		pwForce = false;
		pwOpen = true;
	}

	async function submitPassword(e: Event) {
		e.preventDefault();
		if (!pwTarget) return;
		pwSaving = true;
		try {
			await changePassword(pwTarget.id, { new_password: pwNew, force_logout: pwForce });
			toastSuccess($_('settings.users.password_reset'));
			pwOpen = false;
		} catch (err) {
			toastError(err);
		} finally {
			pwSaving = false;
		}
	}

	function openDelete(u: UserRecord) {
		delTarget = u;
		delOpen = true;
	}

	async function submitDelete() {
		if (!delTarget) return;
		delSaving = true;
		try {
			await deleteUser(delTarget.id);
			toastSuccess($_('settings.users.deleted'));
			delOpen = false;
			await refresh();
		} catch (err) {
			toastError(err);
		} finally {
			delSaving = false;
		}
	}

	function openReset2FA(u: UserRecord) {
		resetTarget = u;
		resetOpen = true;
	}

	async function submitReset2FA() {
		if (!resetTarget) return;
		resetSaving = true;
		try {
			await adminReset2FA(resetTarget.id);
			toastSuccess($_('settings.users.twofa_reset_done'));
			resetOpen = false;
			await refresh();
		} catch (err) {
			toastError(err);
		} finally {
			resetSaving = false;
		}
	}

	function canDelete(u: UserRecord): boolean {
		if (!me) return false;
		if (u.id === me.id) return false;
		if (u.role === 'admin' && adminCount <= 1) return false;
		return true;
	}

	function relativeTime(iso: string): string {
		const then = new Date(iso).getTime();
		const now = Date.now();
		const sec = Math.max(0, Math.floor((now - then) / 1000));
		if (sec < 60) return `${sec}s`;
		if (sec < 3600) return `${Math.floor(sec / 60)}m`;
		if (sec < 86400) return `${Math.floor(sec / 3600)}h`;
		return `${Math.floor(sec / 86400)}d`;
	}
</script>

<div class="mb-4 flex items-center justify-between">
	<h2 class="text-base font-medium">{$_('settings.users.title')}</h2>
	<Button onclick={openCreate}>{$_('settings.users.add')}</Button>
</div>

{#if loading}
	<p class="text-sm text-muted-foreground">{$_('common.loading')}</p>
{:else}
	<Table.Root>
		<Table.Header>
			<Table.Row>
				<Table.Head>{$_('settings.users.col_name')}</Table.Head>
				<Table.Head>{$_('settings.users.col_email')}</Table.Head>
				<Table.Head>{$_('settings.users.col_role')}</Table.Head>
				<Table.Head>{$_('settings.users.col_2fa')}</Table.Head>
				<Table.Head>{$_('settings.users.col_created')}</Table.Head>
				<Table.Head class="w-12"></Table.Head>
			</Table.Row>
		</Table.Header>
		<Table.Body>
			{#each users as u (u.id)}
				<Table.Row>
					<Table.Cell>
						<div class="flex flex-col">
							<span class="text-foreground">{u.display_name}</span>
							<span class="text-xs text-muted-foreground">{u.username}</span>
						</div>
					</Table.Cell>
					<Table.Cell>{u.email}</Table.Cell>
					<Table.Cell>
						<span class="rounded border border-border px-2 py-0.5 text-xs">{u.role}</span>
					</Table.Cell>
					<Table.Cell>
						{#if u.totp_enabled}
							<span class="rounded border border-border px-2 py-0.5 text-xs"
								>{$_('settings.users.twofa_on')}</span
							>
						{:else}
							<span class="text-xs text-muted-foreground">{$_('settings.users.twofa_off')}</span>
						{/if}
					</Table.Cell>
					<Table.Cell>{relativeTime(u.created_at)}</Table.Cell>
					<Table.Cell>
						<DropdownMenu>
							<DropdownMenuTrigger>
								{#snippet child({ props })}
									<Button {...props} variant="ghost" size="icon-sm" aria-label={$_('common.actions')}>
										<IconDotsVertical size={14} />
									</Button>
								{/snippet}
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end">
								<DropdownMenuItem onclick={() => openEdit(u)}>
									{$_('common.edit')}
								</DropdownMenuItem>
								<DropdownMenuItem onclick={() => openPassword(u)}>
									{$_('settings.users.reset_password')}
								</DropdownMenuItem>
								<DropdownMenuItem
									disabled={!u.totp_enabled || u.id === me?.id}
									onclick={() => openReset2FA(u)}
								>
									{$_('settings.users.reset_2fa')}
								</DropdownMenuItem>
								<DropdownMenuItem disabled={!canDelete(u)} onclick={() => openDelete(u)}>
									{$_('common.delete')}
								</DropdownMenuItem>
							</DropdownMenuContent>
						</DropdownMenu>
					</Table.Cell>
				</Table.Row>
			{/each}
		</Table.Body>
	</Table.Root>
{/if}

<Dialog bind:open={createOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('settings.users.create_title')}</DialogTitle>
		</DialogHeader>
		<form onsubmit={submitCreate} class="space-y-3">
			<div class="space-y-1.5">
				<Label for="cu-user">{$_('settings.users.username')}</Label>
				<Input id="cu-user" bind:value={cUsername} required minlength={3} maxlength={64} />
			</div>
			<div class="space-y-1.5">
				<Label for="cu-email">{$_('settings.users.email')}</Label>
				<Input id="cu-email" type="email" bind:value={cEmail} required />
			</div>
			<div class="space-y-1.5">
				<Label for="cu-display">{$_('settings.users.display_name')}</Label>
				<Input id="cu-display" bind:value={cDisplay} required maxlength={100} />
			</div>
			<div class="space-y-1.5">
				<Label for="cu-pw">{$_('settings.users.password')}</Label>
				<Input id="cu-pw" type="password" bind:value={cPassword} required minlength={12} />
			</div>
			<div class="space-y-1.5">
				<Label for="cu-role">{$_('settings.users.role')}</Label>
				<Select.Root type="single" bind:value={cRole}>
					<Select.Trigger id="cu-role" class="w-full">{cRole}</Select.Trigger>
					<Select.Content>
						<Select.Item value="admin">admin</Select.Item>
						<Select.Item value="editor">editor</Select.Item>
						<Select.Item value="viewer">viewer</Select.Item>
					</Select.Content>
				</Select.Root>
			</div>
			<DialogFooter>
				<Button type="button" variant="outline" onclick={() => (createOpen = false)}>
					{$_('common.cancel')}
				</Button>
				<Button type="submit" disabled={cSaving}>
					{cSaving ? $_('common.saving') : $_('common.save')}
				</Button>
			</DialogFooter>
		</form>
	</DialogContent>
</Dialog>

<Dialog bind:open={editOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('settings.users.edit_title')}</DialogTitle>
		</DialogHeader>
		{#if editTarget}
			{@const isSelf = me?.id === editTarget.id}
			<form onsubmit={submitEdit} class="space-y-3">
				<div class="space-y-1.5">
					<Label for="eu-email">{$_('settings.users.email')}</Label>
					<Input id="eu-email" type="email" bind:value={eEmail} required />
				</div>
				<div class="space-y-1.5">
					<Label for="eu-display">{$_('settings.users.display_name')}</Label>
					<Input id="eu-display" bind:value={eDisplay} required maxlength={100} />
				</div>
				<div class="space-y-1.5">
					<Label for="eu-role">{$_('settings.users.role')}</Label>
					<Select.Root type="single" bind:value={eRole} disabled={isSelf}>
						<Select.Trigger id="eu-role" class="w-full">{eRole}</Select.Trigger>
						<Select.Content>
							<Select.Item value="admin">admin</Select.Item>
							<Select.Item value="editor">editor</Select.Item>
							<Select.Item value="viewer">viewer</Select.Item>
						</Select.Content>
					</Select.Root>
					{#if isSelf}
						<p class="text-xs text-muted-foreground">
							{$_('settings.users.cannot_change_own_role')}
						</p>
					{/if}
				</div>
				<DialogFooter>
					<Button type="button" variant="outline" onclick={() => (editOpen = false)}>
						{$_('common.cancel')}
					</Button>
					<Button type="submit" disabled={eSaving}>
						{eSaving ? $_('common.saving') : $_('common.save')}
					</Button>
				</DialogFooter>
			</form>
		{/if}
	</DialogContent>
</Dialog>

<Dialog bind:open={pwOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('settings.users.reset_password')}</DialogTitle>
			<DialogDescription>
				{pwTarget ? pwTarget.display_name : ''}
			</DialogDescription>
		</DialogHeader>
		<form onsubmit={submitPassword} class="space-y-3">
			<div class="space-y-1.5">
				<Label for="pw-new">{$_('settings.users.new_password')}</Label>
				<Input id="pw-new" type="password" bind:value={pwNew} required minlength={12} />
			</div>
			<label class="flex items-center gap-2 text-sm">
				<input type="checkbox" bind:checked={pwForce} class="size-4" />
				{$_('settings.users.force_logout')}
			</label>
			<DialogFooter>
				<Button type="button" variant="outline" onclick={() => (pwOpen = false)}>
					{$_('common.cancel')}
				</Button>
				<Button type="submit" disabled={pwSaving}>
					{pwSaving ? $_('common.saving') : $_('common.save')}
				</Button>
			</DialogFooter>
		</form>
	</DialogContent>
</Dialog>

<Dialog bind:open={resetOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('settings.users.reset_2fa_title')}</DialogTitle>
			<DialogDescription>
				{$_('settings.users.reset_2fa_confirm', {
					values: { name: resetTarget?.display_name ?? '' }
				})}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button type="button" variant="outline" onclick={() => (resetOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={submitReset2FA} disabled={resetSaving}>
				{resetSaving ? $_('common.saving') : $_('settings.users.reset_2fa')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>

<Dialog bind:open={delOpen}>
	<DialogContent>
		<DialogHeader>
			<DialogTitle>{$_('settings.users.delete_title')}</DialogTitle>
			<DialogDescription>
				{$_('settings.users.delete_confirm', {
					values: { name: delTarget?.display_name ?? '' }
				})}
			</DialogDescription>
		</DialogHeader>
		<DialogFooter>
			<Button type="button" variant="outline" onclick={() => (delOpen = false)}>
				{$_('common.cancel')}
			</Button>
			<Button variant="destructive" onclick={submitDelete} disabled={delSaving}>
				{delSaving ? $_('common.deleting') : $_('common.delete')}
			</Button>
		</DialogFooter>
	</DialogContent>
</Dialog>
