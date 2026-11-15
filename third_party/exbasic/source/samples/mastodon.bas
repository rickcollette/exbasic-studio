' A Mastodon Client in exBASIC

' N: Unit to use
unit=8

' URL to Mastodon Server
url$="N:HTTPS://oldbytes.space/api/v1/timelines/public?limit=1"$9B

' QUERY string
query$=""

' QUERY result
DIM result(1024) BYTE

' JSON channel mode
JSON_MODE=1

' PROCEDURES '''''''''''''''''''''''''
FUNC nprinterror()
 NSTATUS unit
 PRINT "ERROR- "; PEEK($02ED)
ENDFUNC

FUNC nsetchannelmode(mode)
 SIO $71, unit, $FC, $00, 0, $1F, 0, 12, JSON_MODE
ENDFUNC

FUNC nparsejson()
 SIO $71, unit, $50, $00, 0, $1f, 0, 12, 0
ENDFUNC

FUNC njsonquery()
 SIO $71, unit, $51, $80, &query$+1, $1f, 256, 12, 0
ENDFUNC

FUNC showresult()
 njsonquery()
 NSTATUS unit

 IF PEEK($02ED) > 128
  PRINT "Could not fetch query:"
  PRINT query$
  EXIT
 ENDIF

 BW=DPEEK($02EA)
 NGET unit, &result, BW
 BPUT #0, &result, BW
ENDFUNC

FUNC mastodon()
 ' Open connection
 NOPEN unit, 12, 0, url$

 ' If not successful, then exit.
 IF SERR()<>1
  PRINT "Could not open connection."
  nprinterror()
  EXIT
 ENDIF

 ' Change channel mode to JSON
 nsetchannelmode(JSON_MODE)

 ' Ask FujiNet to parse JSON
 nparsejson()

 ' If not successful, then exit.
 IF SErr()<>1
  PRINT "Could not parse JSON."
  nprinterror()
  EXIT
 ENDIF

 ' Show latest post
 query$="N:/0/account/display_name"$9B
 showresult()
 query$="N:/0/created_at"$9B
 showresult()
 query$="N:/0/content"$9B
 showresult()

 NCLOSE unit

 PRINT
 PRINT " ---- "
 PRINT

ENDFUNC

' MAIN PROGRAM '''''''''''''''''''''''
DO
 mastodon()
 PAUSE 1800
LOOP
