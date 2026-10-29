# Security policy

## What is in scope

This repository: the compiler (`deskosctl`), the workflows that scan, sign and
publish, and the Core images and installer ISO they produce.

## What is not

- **The base distribution.** A vulnerable package in `centos-bootc` or its RPM
  repositories belongs to the distribution; report it there. The weekly build's
  exceptions list names the ones Core accepts and says why.
- **RHEL-derived artifacts.** The RHEL workstation, its installer and its signing
  key belong to the organization that builds them, and are never published here.
- **An organization's resource root.** Its maintainer owns what it composes.
- **A compromised maintainer account.** Reproducing that is incident response,
  not a vulnerability in this code.

## How to report

Use GitHub's **private vulnerability reporting** on the *Security* tab, *Report a
vulnerability*. It opens a private advisory only maintainers see. If the form is
missing, ask for a private channel in an issue with no details.

Never put details, a credential, a signing key or installer in a public issue.

## What to expect

- An acknowledgement as soon as a maintainer reads it. There is no SLA and no bug
  bounty: this is a small project.
- A coordinated disclosure: we agree a date, fix or mitigate, then publish. An
  issue in the base distribution goes upstream, and we say so. Credit unless you
  ask us not to.

## Verifying what we publish

The security properties of a published image are in
[supply-chain.md](docs/supply-chain.md): the SBOM, the scan and its accepted
risks, the keyless signature, and the records keyed by digest. A digest whose
records do not verify is not promoted, and `:stable` needs a recorded approval.
