# Security policy

## What is in scope

This repository: the compiler (`deskosctl`), the workflows that scan, sign and
publish, and the Core images and installer ISO those workflows produce for the
community platform.

## What is not

- **The base distribution.** A vulnerable package in `centos-bootc` or its
  RPM repositories belongs to the distribution; report it there. The gate's
  exceptions record names the ones Core accepts and says why.
- **RHEL-derived artifacts.** The RHEL workstation, its installer and its
  signing key belong to the organization that builds them, and are never
  published here.
- **An organization's resource root.** Its maintainer owns what it composes.
- **Anything that needs a compromised maintainer account** to reproduce; that
  is incident response, not a vulnerability in this code.

## How to report

Use GitHub's **private vulnerability reporting**: the repository's *Security*
tab → *Report a vulnerability*. It opens a private advisory that only the
maintainers see.

If the form is not available, open an issue that says only that you need a
private channel, with no details, and wait for a reply.

Please do not open a public issue with the details, and do not attach a
credential, a signing key or an internal installer.

## What to expect

- An acknowledgement as soon as a maintainer reads it. There is no SLA and no
  bug bounty: this is a small project, and saying otherwise would be a promise
  nobody can keep.
- A coordinated disclosure: we agree a date with you, fix or mitigate, then
  publish the advisory. If the issue is in the base distribution, we route it
  upstream and say so.
- Credit in the advisory unless you ask us not to.

## Verifying what we publish

The security properties of a published image are described in
[supply-chain.md](docs/supply-chain.md): the SBOM, the scan and its accepted
risks, the keyless signature, and the evidence keyed by digest. A digest whose
evidence does not verify is not promoted, and `:stable` only moves with a
recorded approval.
