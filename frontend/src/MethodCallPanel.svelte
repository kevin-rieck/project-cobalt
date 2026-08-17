<script lang="ts">
  import { onDestroy, onMount } from 'svelte'
  import { CallMethod, GetMethodDetails } from '../wailsjs/go/main/App.js'
  import { isSupportedScalarDataType, parseScalarInputError } from './scalarInput'

  type AddressNode = { ParentNodeID: string; NodeID: string; DisplayName: string; BrowseName: string; NodeClass: string }
  type MethodArgument = { Name: string; DataType: string; DataTypeID: string; ValueRank: string; Description: string; ArrayDimensions: number[]; Supported: boolean }
  type MethodDetails = { ObjectNodeID: string; MethodNodeID: string; Description: string; Executable: boolean; UserExecutable: boolean; InputArguments: MethodArgument[]; OutputArguments: MethodArgument[] }
  type MethodCallResult = { StatusCode: string; InputArgumentResults: string[]; OutputArguments: Array<{ DataType: string; Value: string }> }
  type MethodCallAvailability = { connected: boolean; readOnlyMode: boolean; details: MethodDetails | null; detailsLoading: boolean; detailsError: string; inputs: string[]; submitting: boolean }

  export let selectedMethod: AddressNode
  export let objectNodeName: string
  export let sessionName: string
  export let connected: boolean
  export let readOnlyMode: boolean
  export let addToast: (level: string, message: string) => void

  let details: MethodDetails | null = null
  let detailsLoading = true
  let detailsError = ''
  let inputs: string[] = []
  let submitting = false
  let result: MethodCallResult | null = null
  let callError = ''
  let disposed = false

  $: disabledReasons = methodCallDisabledReasons({ connected, readOnlyMode, details, detailsLoading, detailsError, inputs, submitting })
  $: canCall = !!details && disabledReasons.length === 0

  onMount(async () => {
    try {
      const loaded = await GetMethodDetails({ objectNodeID: selectedMethod.ParentNodeID || '', methodNodeID: selectedMethod.NodeID })
      if (disposed) return
      details = { ...loaded, InputArguments: loaded.InputArguments || [], OutputArguments: loaded.OutputArguments || [] }
      inputs = details.InputArguments.map(() => '')
    } catch (error) {
      if (!disposed) detailsError = String(error)
    } finally {
      if (!disposed) detailsLoading = false
    }
  })

  onDestroy(() => { disposed = true })

  function updateInput(index: number, value: string) {
    inputs = inputs.map((current, currentIndex) => currentIndex === index ? value : current)
    result = null
    callError = ''
  }

  function methodCallDisabledReasons(state: MethodCallAvailability) {
    const reasons: string[] = []
    if (!state.connected) reasons.push('Connect to an OPC UA Server before calling a Method.')
    if (state.readOnlyMode) reasons.push('Read-Only Mode is active.')
    if (state.detailsLoading) reasons.push('Method metadata is loading.')
    if (state.detailsError) reasons.push(`Method metadata failed to load: ${state.detailsError}`)
    if (!state.details && !state.detailsLoading && !state.detailsError) reasons.push('Method metadata is unavailable.')
    if (!state.details) return reasons
    if (!state.details.Executable) reasons.push('Method is not executable.')
    else if (!state.details.UserExecutable) reasons.push('Method is not executable for the current session.')
    state.details.InputArguments.forEach((argument, index) => {
      const unsupportedType = !isSupportedScalarDataType(argument.DataType)
      const unsupportedRank = argument.ValueRank !== 'Scalar'
      if (!argument.Supported || unsupportedType || unsupportedRank) {
        const limitations = [unsupportedType ? `DataType ${argument.DataType}` : '', unsupportedRank ? `ValueRank ${argument.ValueRank}` : ''].filter(Boolean).join(' and ')
        reasons.push(`${argument.Name || `Input ${index + 1}`} uses unsupported ${limitations || 'argument metadata'}.`)
        return
      }
      const error = parseScalarInputError(argument.DataType, state.inputs[index] || '', argument.Name || `Input ${index + 1}`)
      if (error) reasons.push(error)
    })
    if (state.submitting) reasons.push('Method call is in progress.')
    return reasons
  }

  async function submit() {
    if (!details || !canCall || submitting || !connected) return
    submitting = true
    result = null
    callError = ''
    try {
      const response = await CallMethod({ objectNodeID: details.ObjectNodeID, methodNodeID: details.MethodNodeID, inputArguments: inputs })
      if (disposed) return
      result = { ...response, InputArgumentResults: response.InputArgumentResults || [], OutputArguments: response.OutputArguments || [] }
      if (response.StatusCode.includes('Good')) addToast('info', `Method call completed: ${response.StatusCode}`)
      else addToast('error', `Method call returned ${response.StatusCode}`)
    } catch (error) {
      if (disposed) return
      callError = `Method call failed: ${String(error)}`
      addToast('error', callError)
    } finally {
      if (!disposed) submitting = false
    }
  }
</script>

{#if detailsLoading}
  <div class="rounded border border-outline-variant bg-surface-container-low p-md text-on-surface-variant">Loading Method metadata…</div>
{/if}
{#if details}
  <div class="space-y-lg">
    <div>
      <div class="flex flex-wrap items-center gap-sm">
        <span class="material-symbols-outlined text-primary">play_circle</span>
        <span class="status-chip border-primary/40 text-primary">Method Node</span>
        <span class="status-chip {details.Executable && details.UserExecutable ? 'border-emerald-400/40 text-emerald-400' : 'border-tertiary-container/50 text-tertiary'}">{details.Executable && details.UserExecutable ? 'Executable for this session' : details.Executable ? 'Not executable for this session' : 'Not executable'}</span>
      </div>
      <p class="mt-md text-on-surface-variant">{details.Description || 'No description provided.'}</p>
    </div>

    <dl class="grid gap-sm text-sm sm:grid-cols-2">
      <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Object Node</dt><dd class="mt-xs font-semibold">{objectNodeName}</dd></div>
      <div class="rounded border border-outline-variant bg-surface-container-low p-sm"><dt class="label">Method Node</dt><dd class="mt-xs font-semibold">{selectedMethod.DisplayName}</dd></div>
      <div class="rounded border border-outline-variant bg-surface-container-low p-sm sm:col-span-2"><dt class="label">Object NodeID</dt><dd class="mt-xs break-all font-mono">{details.ObjectNodeID}</dd></div>
      <div class="rounded border border-outline-variant bg-surface-container-low p-sm sm:col-span-2"><dt class="label">Method NodeID</dt><dd class="mt-xs break-all font-mono">{details.MethodNodeID}</dd></div>
    </dl>

    <section aria-labelledby="method-input-heading">
      <div class="flex items-center justify-between"><h3 id="method-input-heading" class="text-lg font-semibold">Input arguments</h3><span class="font-mono text-xs text-on-surface-variant">{details.InputArguments.length} ordered</span></div>
      {#if details.InputArguments.length === 0}
        <p class="mt-sm text-sm text-on-surface-variant">This Method has no input arguments.</p>
      {:else}
        <div class="mt-sm space-y-sm">
          {#each details.InputArguments as argument, index}
            <label class="block rounded border border-outline-variant bg-surface-container-low p-md">
              <span class="flex flex-wrap items-center justify-between gap-sm"><span class="font-semibold">{argument.Name || `Input ${index + 1}`}</span><span class="font-mono text-xs text-primary">{argument.DataType} · {argument.ValueRank}</span></span>
              <input class="field mt-sm w-full" aria-label={argument.Name || `Input ${index + 1}`} value={inputs[index] || ''} disabled={!argument.Supported || argument.ValueRank !== 'Scalar' || !isSupportedScalarDataType(argument.DataType)} on:input={(event) => updateInput(index, event.currentTarget.value)} on:keydown={(event) => event.key === 'Enter' && event.preventDefault()} placeholder={`Enter ${argument.DataType}`} />
              <span class="mt-sm block text-xs text-on-surface-variant">DataType NodeID: {argument.DataTypeID || '—'} · Dimensions: {argument.ArrayDimensions?.length ? argument.ArrayDimensions.join(' × ') : 'none'}</span>
              {#if argument.Description}<span class="mt-xs block text-sm text-on-surface-variant">{argument.Description}</span>{/if}
            </label>
          {/each}
        </div>
      {/if}
    </section>

    <section aria-labelledby="method-output-heading">
      <div class="flex items-center justify-between"><h3 id="method-output-heading" class="text-lg font-semibold">Output arguments</h3><span class="font-mono text-xs text-on-surface-variant">{details.OutputArguments.length} ordered</span></div>
      {#if details.OutputArguments.length === 0}
        <p class="mt-sm text-sm text-on-surface-variant">This Method has no declared output arguments.</p>
      {:else}
        <ol class="mt-sm space-y-sm">
          {#each details.OutputArguments as argument, index}
            <li class="rounded border border-outline-variant bg-surface-container-low p-sm text-sm"><span class="font-semibold">{index + 1}. <span>{argument.Name || `Output ${index + 1}`}</span></span><span class="ml-sm font-mono text-xs text-primary">{argument.DataType} · {argument.ValueRank}</span><p class="mt-xs text-xs text-on-surface-variant">DataType NodeID: {argument.DataTypeID || '—'} · Dimensions: {argument.ArrayDimensions?.length ? argument.ArrayDimensions.join(' × ') : 'none'}</p>{#if argument.Description}<p class="mt-xs text-on-surface-variant">{argument.Description}</p>{/if}</li>
          {/each}
        </ol>
      {/if}
    </section>

    {#if disabledReasons.length > 0}
      <ul class="list-disc space-y-xs pl-lg text-sm text-on-surface-variant">{#each disabledReasons as reason}<li>{reason}</li>{/each}</ul>
    {/if}
    <section class="rounded border border-tertiary-container/60 bg-tertiary-container/10 p-md" aria-label="Method call review">
      <p class="label">Review Method call</p>
      <dl class="mt-sm space-y-xs text-sm">
        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Saved Connection / Endpoint</dt><dd class="text-right font-mono">{sessionName}</dd></div>
        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Object Node</dt><dd class="text-right">{objectNodeName}</dd></div>
        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Method Node</dt><dd class="text-right">{selectedMethod.DisplayName}</dd></div>
        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Object NodeID</dt><dd class="break-all text-right font-mono">{details.ObjectNodeID}</dd></div>
        <div class="flex justify-between gap-md"><dt class="text-on-surface-variant">Method NodeID</dt><dd class="break-all text-right font-mono">{details.MethodNodeID}</dd></div>
        {#each details.InputArguments as argument, index}<div class="flex justify-between gap-md"><dt class="text-on-surface-variant">{argument.Name || `Input ${index + 1}`}</dt><dd class="break-all text-right font-mono">{inputs[index] || '—'}</dd></div>{/each}
      </dl>
    </section>
    <button class="btn-primary w-full" disabled={!canCall} on:click={submit}>{submitting ? 'Calling…' : 'Call Method'}</button>

    {#if callError}<div class="rounded border border-error-container bg-error-container/20 p-md text-error">{callError}</div>{/if}
    {#if result}
      <section class="rounded border {result.StatusCode.includes('Good') ? 'border-primary/50 bg-primary/10' : 'border-error-container bg-error-container/20'} p-md" aria-label="Method call result">
        <p class="label">StatusCode</p><p class="mt-xs break-all font-mono text-lg">{result.StatusCode}</p>
        {#if result.InputArgumentResults.length}<p class="mt-md label">Input argument results</p><ol class="mt-xs list-decimal pl-lg font-mono text-sm">{#each result.InputArgumentResults as status}<li>{status}</li>{/each}</ol>{/if}
        <p class="mt-md label">Raw outputs</p>
        {#if result.OutputArguments.length}<ol class="mt-xs space-y-xs">{#each result.OutputArguments as output, index}<li class="font-mono text-sm">{index + 1}. <span class="text-primary">{output.DataType}</span> <span>{output.Value}</span></li>{/each}</ol>{:else}<p class="mt-xs text-sm text-on-surface-variant">No output values returned.</p>{/if}
      </section>
    {/if}
  </div>
{/if}
{#if !details}
  <button class="btn-primary mt-md w-full" disabled>Call Method</button>
  {#if disabledReasons.length > 0}<ul class="mt-md list-disc space-y-xs pl-lg text-sm text-on-surface-variant">{#each disabledReasons as reason}<li>{reason}</li>{/each}</ul>{/if}
{/if}
