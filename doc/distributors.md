Bridge distribution mechanisms
==============================

Rdsys implements various mechanisms to distribute bridges. The following list briefly explains how these mechanisms work.

Settings
--------

The "Settings" distributor is used by Tor Browser and other clients to autoconfigure the circumvention settings depending on the location of the user. It uses a map of countries and what circumvention mechanism works in each to provide the right kind of bridge for each country. The [Circumvention Settings API is part of moat](moat.md).

HTTPS
-----

The "HTTPS" distribution mechanism hands out bridges over this website. To get bridges, go to [bridges.torproject.org](https://bridges.torproject.org), select your preferred options.

Email
-----

Users can request bridges from the "Email" distribution mechanism by sending an email to bridges@torproject.org and writing "get transport obfs4" in the email body.

Telegram
--------

Users can request bridges from the ["Telegram" distribution mechanism](telegram.md) by sending the '/bridges' command to [@GetBridgesBot](https://t.me/GetBridgesBot) over the Telegram instant messaging network.

Lox
---

["Lox"](https://gitlab.torproject.org/tpo/anti-censorship/lox) is a privacy preserving reputation-based bridge distribution mechanism. It's currently under development.

Users can request an invitation to Lox which will provide access to a single bridge from the ["Telegram" distribution mechanism](telegram.md) by sending the '/lox' command to [@GetBridgesBot](https://t.me/GetBridgesBot) over the Telegram instant messaging network and pasting the resulting string into Tor browser.

Moat
----

The ["Moat" distribution mechanism](moat.md) is part of Tor Browser, allowing users to request bridges from inside their Tor Browser settings. To get bridges, go to your Tor Browser's Tor settings, click on "request a new bridge", solve the subsequent CAPTCHA, and Tor Browser will automatically add your new bridges. This mechanism is [being deprecated in favor of the "Settings" distributor](https://gitlab.torproject.org/tpo/applications/tor-browser/-/issues/42086).

Reserved
--------

Rdsys maintains a small number of bridges that are not distributed automatically. Instead, we reserve these bridges for manual distribution and hand them out to NGOs and other organizations and individuals that need bridges. Bridges that are distributed over the "Reserved" mechanism may not see users for a long time. Note that the "Reserved" distribution mechanism was previously called "Unallocated" in bridge pool assignment files.

None
----

Bridges that have a distribution mechanism of "None" are not distributed by Rdsys. It is the bridge operator's responsibility to distribute their bridges to users. Note that on Relay Search, a freshly set up bridge's distribution mechanism says "None" for up to approximately one day. Be a bit patient, and it will then change to the bridge's actual distribution mechanism.
