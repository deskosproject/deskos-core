# Satellite and Foreman as a DeskOS content source

**Status:** research, 2026-11-09. This note answers issue #466: whether a
Satellite-style source can serve the bootc image, the CS10 repositories and a
few Flatpaks to the lifecycle test, and whether it fits a GitHub-hosted runner.

## What exists today

- **Satellite 6.20 limited availability** is the container-based deployment on
  RHEL 10: application components run in Podman containers managed by Quadlets.
  It is a fresh install, not an in-place upgrade; general availability is 2027.
- **Foreman/Katello containerized** is available upstream through `foremanctl`
  on EL9 and EL10. The current documentation includes a container registry and
  native **Flatpak remotes**, so the same primitives exist without waiting for
  Satellite 6.20 GA.
- The Foreman container registry is an OCI registry. A host pulls images from
  `foreman.example.com/<organization>/<product>/<repository>:<tag>` (or the
  ID-based equivalent), after trusting the Foreman CA and logging in.
- Flatpak repositories in Foreman are `docker` content. A host adds the server
  as an OCI Flatpak remote
  (`flatpak remote-add --authenticator-name=org.flatpak.Authenticator.Oci ...`)
  and installs applications after `podman login`. The published CS10 image ships
  Flatpak 1.18 and `/usr/libexec/flatpak-oci-authenticator`, so the client side
  is present.

So the protocols DeskOS needs are present: OCI for bootc, yum for CS10, and OCI
for Flatpak.

## Minimum footprint

Upstream Foreman/Katello containerized documents:

| Resource | Minimum |
|---|---|
| CPU | 4 cores |
| RAM | 20 GB, plus 4 GB swap |
| `/var/lib/pulp` | 300 GB runtime |
| `/var/lib/containers` | 30 GB runtime |
| `/var/lib/pgsql` | 20 GB runtime |
| `/usr` | 10 GB |

The default tuning profile (0 to 5,000 hosts) is 20 GB RAM and 4 cores. Satellite
6.20 LA uses the same container architecture; no smaller supported profile was
found.

A GitHub-hosted **standard** Linux runner has 4 vCPU, 16 GB RAM and a 14 GB SSD.
That does not meet the RAM requirement and is far below the Pulp storage figure.
A **larger** GitHub-hosted runner (16 vCPU, 64 GB RAM, 600 GB disk) would fit,
but it is not a standard runner; using it needs a decision under the repository's
runner policy.

## What the pipeline would run

1. `foremanctl deploy` on EL10 with a stable FQDN, SELinux enabled, a CA and an
   initial organization.
2. `hammer` configuration:
   - CS10 product and `yum` repositories with `--download-policy on_demand`;
   - a `docker` repository for the DeskOS image, synced from its registry;
   - a Flatpak remote for the Fedora or Red Hat index, then
     `hammer flatpak-remote remote-repository mirror`; mirror the matching
     runtime repository as well, because an application such as Firefox
     depends on `org.fedoraproject.Platform` or the RHEL `flatpak-runtime`;
   - a content view, published and promoted to a lifecycle environment.
3. `hammer repository synchronize` for each repository.
4. On the guest: trust the Foreman CA, `podman login`, point bootc at the
   Foreman registry, and add the Flatpak remote as
   `oci+https://foreman.example.com/`.

The on-demand policy applies to **yum** repositories: metadata is synchronized
and packages are fetched on request. Container images and Flatpak runtimes are
layers; their sync downloads the layers, so the image and the chosen Flatpak
runtime still consume real storage.

## Hammer shape

The exact IDs come from `hammer ... list`; the shape is:

```bash
hammer product create --name "CentOS Stream 10" --organization "ORG"
hammer repository create --product "CentOS Stream 10" --name BaseOS \
  --content-type yum --url https://mirror.stream.centos.org/10-stream/BaseOS/x86_64/os/ \
  --download-policy on_demand --organization "ORG"
hammer repository synchronize --product "CentOS Stream 10" --name BaseOS --organization "ORG"

hammer repository create --product "DeskOS" --name deskos-core-cs10 \
  --content-type docker --docker-upstream-name deskos/deskos-core-centos-stream-10 \
  --url https://quay.io/ --organization "ORG"
hammer repository synchronize --product "DeskOS" --name deskos-core-cs10 --organization "ORG"

hammer flatpak-remote create --name fedora --organization-id 1 \
  --url https://registry.fedoraproject.org/
hammer flatpak-remote scan --id 1
hammer flatpak-remote remote-repository mirror --flatpak-remote-id 1 \
  --id REPOSITORY_ID --product-id PRODUCT_ID
```

A content view then publishes the repositories and promotes the version to the
lifecycle environment the hosts consume.

## Is it technically possible?

Yes, for the client path:

- bootc can pull an OCI image from the Foreman registry;
- Katello can mirror CS10 as an on-demand yum source;
- Foreman's Flatpak remotes are OCI repositories, and a host can add them with
  the OCI Flatpak authenticator (`org.flatpak.Authenticator.Oci`); the client
  must provide that authenticator and trust the Foreman CA.

The blockers are operational, not protocol-level:

- **Runner memory and disk.** A standard runner cannot host the documented
  Foreman/Katello minimum. This is the main reason not to put the full server in
  the pull-request or weekly lifecycle workflow.
- **TLS, DNS and auth.** Foreman uses a CA, a FQDN and host-header validation.
  The guest needs the CA, an `/etc/hosts` entry or DNS, and container auth for
  bootc and Flatpak. These are solvable but add setup and failure modes.
- **Time.** Pulling the Foreman/Katello containers, deploying, syncing CS10
  metadata and synchronizing the image and Flatpak layers adds roughly 15 to 30
  minutes before the already 45 to 60 minute lifecycle run starts.
- **Product status.** Satellite 6.20 is limited availability, and Foreman
  nightly is not the supported product. A CI fixture should pin a specific
  version and accept that it is an integration test, not a supported
  deployment.

## Options

1. **Lab host or larger runner.** Run the full Foreman/Katello and point the
   lifecycle driver at it. This proves the Satellite path end to end, but it is a
   dedicated weekly or manual job, not a standard-runner check.
2. **Small stand-in in CI.** Keep `registry:2` or a small OCI registry and add a
   static Flatpak OCI repository. This exercises the same client path (bootc
   pull, Flatpak OCI remote, TLS/auth) without Pulp, PostgreSQL or the Foreman
   stack. Issue #466 explicitly allows a stand-in that speaks the same
   protocols.
3. **Organization-owned content source.** Leave the full Satellite deployment
   where it belongs and run the lifecycle test against it downstream. Core keeps
   a fixture and the resource shape; the organization owns the server and the
   subscription.

## Recommendation

Do not add full Foreman/Katello to a standard GitHub-hosted runner. The
documented minimum is 20 GB RAM and hundreds of GB of Pulp storage; the standard
runner has 16 GB and a 14 GB SSD.

Use the full server for a lab or a larger runner, and use a small stand-in for
the CI fixture. The model work that matters first is the resource shape:
`RpmRepository` for the content source, `TrustAnchor` for the CA,
`FlatpakRemote` for the OCI Flatpak remote, and a bootc image reference that
points at the registry. That work does not depend on which server provides it.

## Where the work lives

Core keeps this note, the model contract and a small stand-in fixture. The real
Foreman/Katello deployment includes FQDN, CA, DNS, registry auth, subscription,
content views, the on-demand CS10 mirror and the Flatpak mirror. It lives in the
downstream workstation repository that owns the organization's pipeline. It is
not `deskos-infrastructure`, which holds Core's Cloudflare/R2 resources.

Issue #466 stays open until that downstream run records the full path; Core
closes it with the record, not with a simulated server.

## Sources

- Red Hat, Satellite 6.20 limited availability (2026-09-02):
  <https://www.redhat.com/en/blog/red-hat-satellite-620-limited-availability-early-access-containerized-management-and-post-quantum-cryptographic-enablement>
- Foreman, containerized Katello quickstart (nightly):
  <https://docs.theforeman.org/nightly/Quickstart/index-containerized-katello.html>
- Foreman, installing containerized Katello (system and storage requirements):
  <https://docs.theforeman.org/nightly/Installing_Server/index-containerized-katello.html>
- Foreman, managing container images:
  <https://docs.theforeman.org/nightly/Managing_Content/index-katello.html#manage-container-images>
- Foreman, managing Flatpak repositories:
  <https://docs.theforeman.org/nightly/Managing_Content/index-katello.html#manage-flatpak-repositories-in-foreman>
- Foreman, installing Flatpak applications on hosts:
  <https://docs.theforeman.org/nightly/Managing_Hosts/index-katello.html#install-flatpak-applications-on-hosts>
- GitHub, standard hosted runner specifications:
  <https://docs.github.com/en/actions/reference/runners/github-hosted-runners>
