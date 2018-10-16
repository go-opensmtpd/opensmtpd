# opensmtpd
OpenSMTPD-extras in Go

## WARNING

This is very much Work In Progress. Don't use this for production
installations. Much of the filter API in OpenSMTPD is not yet stable,
so this package is also subject to change.

We have implemented Filter API version 51, because that's compatible with
the most recent portable OpenSMTPD version available in Debian Jessie and
Stretch (version 6.0.2p1).
