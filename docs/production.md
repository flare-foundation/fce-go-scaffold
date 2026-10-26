# Production

> **Work in progress.**
> This is a draft: no production machine has been run this way yet, and every live run so far was a test machine on `TEST_PLATFORM`.
> `fce` helps only with `deploy`, `add-version`, `governance` and `disable-version`; the rest is by hand.
> Each **TODO** stands where nothing has checked the answer yet; [open points](#open-points) lists what is known but untested.

A production machine runs the extension's image on a Google Cloud [Confidential Space](https://cloud.google.com/confidential-computing/confidential-space/docs/confidential-space-overview) VM, tee-node with the image's `MODE=0`, and Confidential Space attests the image's hash.
tee-proxy, Redis and an indexer run outside the TEE, on hosts the [operator](assumptions.md#the-operator) runs, much as in the tooling README's [By hand](https://github.com/flare-foundation/fce-tooling#by-hand), which sets up the same stack for a test machine.
[Architecture](architecture.md) shows how the pieces fit.

## 1. Allow the image

`make image` prints the image's hash, its config digest, and `fce add-version -platform GCP_AMD_SEV,GCP_INTEL_TDX` allows it on the platforms its machines may run on; the list is final for that hash, as the template's [Deploying](../template/README.md#deploying) says.
Anyone can rebuild the image and compare, as [REPRODUCIBILITY.md](../REPRODUCIBILITY.md#verify-a-registered-hash) describes.
Disable the test version, `fce disable-version -test`, before real users arrive: while it is allowed, contracts accept the signatures of [test machines](assumptions.md#test-machines), whose operators hold their keys.

Push the image to a registry the VM can pull from, with `IMAGE` its name there:

```bash
docker tag fce/template "$IMAGE"
docker push "$IMAGE"
```

> **TODO:** the registry, and the access the VM needs to pull from it.

## 2. Allow the machine owner

The machine's `INITIAL_OWNER` registers it, so it must be an allowed machine owner of the extension.
The deployer is; the extension owner allows others with FlareTeeManager's `addAllowedTeeMachineOwners(extensionId, owners)`.

## 3. Run tee-proxy, Redis and an indexer

As in [By hand](https://github.com/flare-foundation/fce-tooling#by-hand), with these differences:

- `[chain] source = "indexer"`, with the `[db]` of a flare-system-c-chain-indexer you run or trust: on a fresh start tee-proxy hands tee-node its first signing policy and the voters' keys unverified, so they are only as trustworthy as their source.
- No `node` or `tunnel` service: tee-node runs on the VM, and the machine's URL must last, since a new one needs `updateTeeMachineSettings`.
- tee-proxy's external port, 6662, behind public HTTPS that doesn't redirect: data providers reach the machine at its registered URL, their relays and verifiers refuse private addresses, and the verifiers follow no redirects.
  tee-proxy speaks plain HTTP, so TLS ends in front of it.
- Its internal port, 6661, published only on the private address the VM reaches: it has no authentication.
- Redis keeping its data: By hand's keeps nothing across a restart, while results stay readable for 14 days by default; tee-proxy's `[gcs]` keeps results in a Cloud Storage bucket instead.
- `PRIVATE_KEY`, a key made for this tee-proxy alone and never funded; its address is the proxy id the machine registers with.
- `[direct] enable = true` only when the extension takes [direct actions](direct-actions.md); then everyone allowed to post them shares `DIRECT_API_KEY`.

> **TODO:** whether to turn on tee-proxy's `[attestation]`.
> Off, tee-proxy trusts whichever TEE answers it first; on, with `expected_code_hashes`, it checks the TEE's attestation token.
> The `expected_platforms` names its example gives match no token.

> **TODO:** whether a production machine needs tee-proxy's `machine_path_manager` and `[governance]`, which `fce` leaves unset.
> They deliver the governance's machine path lists, which authorize wallets' direct backups.

## 4. Launch the VM

Launch a Confidential Space VM, AMD SEV on N2D or Intel TDX on C3, from the production Confidential Space image, never the debug one: a debug image gives its operator root on the VM, and data providers' verifiers refuse it unless set to admit one, `ALLOW_TEE_DEBUG`, which their shipped Coston and Coston2 settings aren't ([what the operator sees](assumptions.md#what-the-operator-sees)).
Its metadata:

- `tee-image-reference`: the pushed image, by the digest `docker push` printed.
  That digest is the manifest's; the hash the machine attests is still the config digest `make image` printed.
- `tee-env-<NAME>` for each variable the launch-policy label in the extension's `Dockerfile` lists: `PROXY_URL`, `INITIAL_OWNER`, `EXTENSION_ID`, `CHAIN_ID`, the governance, and the extension's own; `LOG_LEVEL` is optional, `DEBUG` when unset.
  `PROXY_URL` is tee-proxy's internal API as the VM reaches it, such as `http://<host>:6661`.
  The governance is `GOVERNANCE_SIGNERS` and `GOVERNANCE_THRESHOLD`, as registered on chain, and for a Safe also `GOVERNANCE_SAFE` and `GOVERNANCE_TEE_MANAGER`.
  Set every one that applies: tee-node's [port 5500](architecture.md#ports) takes any left unset from whoever reaches it first.
- `tee-restart-policy`: `Never`, the default.
  A restart reboots the VM, and the next boot is a new machine anyway, with a new key and empty state, to be registered anew ([a TEE lives once](assumptions.md#a-tee-lives-once)).

Close ports 5500 and 6661 to everyone who doesn't need them: whoever reaches 5500 can re-point tee-node at another proxy, at any time, and 6661 has no authentication.
A VPC's internal rules, such as the default network's `default-allow-internal`, open both to every other VM in it.

Only AMD SEV machines on N2D live-migrate through Google's host maintenance, when set to migrate; Intel TDX machines end at every host maintenance, and come back as new machines.

> **TODO:** the `gcloud` command that launches the VM: the machine type, the confidential compute type, the maintenance policy, the production image's project and family, and the service account with the roles it needs.
> `GOVERNANCE_SIGNERS` holds commas, which `--metadata` splits on unless given another separator, as `gcloud topic escaping` describes.

## 5. Check the machine

```bash
curl -s "$PROXY/info" | jq .machineData
```

- `codeHash` is the hash `fce add-version` allowed, never tee-node's test hash `0x194844cf…`, and never compared with the registry's digest.
- `platform` is `GCP_AMD_SEV` or `GCP_INTEL_TDX`, as UTF-8 padded to 32 bytes.
- `extensionId`, `initialOwner` and `governanceHash` match the extension, the owner and the governance on chain.

## 6. Take it to production

From the initial owner, the first four steps the tooling README's [By hand](https://github.com/flare-foundation/fce-tooling#by-hand) lists: `register`, the attestation, the availability check and `toProduction`, with tee-proxy's public URL as the machine's URL.
`updateTeeMachineSettings(teeId, proxyId, url)` changes the proxy or the URL later.

## 7. Keep it in production

An availability check lasts one reward epoch: 6 hours on Coston and Coston2, 3.5 days on Songbird and Flare ([time](assumptions.md#time)).
Renew it every period with a new availability check, whose proof `confirmAvailability` takes.

> **TODO:** the renewal's exact calls: whether it needs a new attestation first, `requestTeeAttestation`, as bringing a paused machine back does.

A machine whose check lapsed stays in production until someone pauses it, which anyone may then do; after a longer lapse it can't be confirmed again.

## 8. Retire or replace it

Pause the machine, `pause(teeId)` from its owner, before stopping its VM: sends keep going to a machine in production, and nothing answers them.
A machine that stops, by a crash, a reboot or host maintenance, comes back as a new one, with a new key, a new `teeId` and empty state, and is registered anew.
An upgrade is a new image with a new hash: allow it with `fce add-version`, start machines on it, then disable the old hash, `fce disable-version -hash <old hash>`, which pauses the old machines for good ([merging upstream](fork.md#merging-upstream)).

## Open points

- The launcher sets the extension's user through `CAP_SETUID`, which Confidential Space grants by its source; that is not yet seen on a production machine ([the image](architecture.md#the-image)).
- A pushed image keeping its config digest is untested; a mismatch shows only at step 5, once the VM runs.
- `confirmAvailability` comes from go-verifier-api's spec and FlareTeeManager's ABI; it was never called.
- No `fce` command registers, renews or retires a production machine yet.
