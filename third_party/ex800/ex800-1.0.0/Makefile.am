ACLOCAL_AMFLAGS = -I m4
AUTOMAKE_OPTIONS = foreign

SUBDIRS = DOC src
if !CONFIGURE_TARGET_ANDROID
SUBDIRS += tools
endif

doc_DATA = COPYING README.TXT

EXTRA_DIST = $(doc_DATA) ex800.spec ex800.nsi act data debian test util
