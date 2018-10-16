/*
Package opensmtpd implements OpenSMTPD APIs


APIs in OpenSMTPD

The APIs in OpenSMTPD are not stable and subject to change. The filter versions
implemented by this package are tested against OpenSMTPD-portable version 6.0.2
(as supplied by Debian Jessie (backports) and Debian Stretch).


Filters

Hooks for the various SMTP transaction stages. If a filter function is
registered for a callback, the OpenSMTPD process expects a reply via the
Session.Accept() or Session.Reject() calls. Failing to do so may result in a
locked up mail server, you have been warned!
*/
package opensmtpd
