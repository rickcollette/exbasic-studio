/*
 * exBASIC Studio headless bridge for Atari800 7.2.1.
 *
 * This program links to libatari800 and is therefore distributed under the
 * GNU GPL version 2 or later, like Atari800 itself. The Studio application
 * communicates with it as a separate process over newline-delimited JSON.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "config.h"
#include "libatari800/libatari800.h"
#include "colours.h"
#include "atari.h"
#include "devices.h"
#include "memory.h"
#include "sio.h"
#ifdef R_IO_DEVICE
#include "rdevice.h"
#endif

#define WIDTH 336
#define HEIGHT 240

static const char base64_table[] =
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

static int host_device_present(void) {
	int address;
	for (address = 0x031a; address < 0x031a + 33; address += 3) {
		if (MEMORY_dGetByte(address) == 'H' && MEMORY_dGetWord(address + 1) == 0xd140)
			return 1;
	}
	return 0;
}

static void write_base64(const unsigned char *data, size_t len) {
	size_t i;
	for (i = 0; i + 2 < len; i += 3) {
		unsigned value = ((unsigned)data[i] << 16) | ((unsigned)data[i + 1] << 8) | data[i + 2];
		putchar(base64_table[(value >> 18) & 63]);
		putchar(base64_table[(value >> 12) & 63]);
		putchar(base64_table[(value >> 6) & 63]);
		putchar(base64_table[value & 63]);
	}
	if (i < len) {
		unsigned value = (unsigned)data[i] << 16;
		putchar(base64_table[(value >> 18) & 63]);
		if (i + 1 < len) {
			value |= (unsigned)data[i + 1] << 8;
			putchar(base64_table[(value >> 12) & 63]);
			putchar(base64_table[(value >> 6) & 63]);
			putchar('=');
		} else {
			putchar(base64_table[(value >> 12) & 63]);
			putchar('=');
			putchar('=');
		}
	}
}

static int request_id(const char *line) {
	const char *value = strstr(line, "\"id\":");
	return value ? atoi(value + 5) : 0;
}

static int hex_digit(char value) {
	if (value >= '0' && value <= '9') return value - '0';
	if (value >= 'a' && value <= 'f') return value - 'a' + 10;
	if (value >= 'A' && value <= 'F') return value - 'A' + 10;
	return -1;
}

/* Decode the small JSON string emitted by encoding/json. This deliberately
 * accepts only the key field and ASCII characters supported by Atari800. */
static int parse_key(const char *line, char *out, size_t capacity) {
	const char *value = strstr(line, "\"key\":\"");
	size_t used = 0;
	if (!value || capacity == 0) return 0;
	value += 7;
	while (*value && *value != '"' && used + 1 < capacity) {
		unsigned char decoded = (unsigned char)*value++;
		if (decoded == '\\') {
			int h0, h1, h2, h3, codepoint;
			decoded = (unsigned char)*value++;
			switch (decoded) {
			case 'b': decoded = '\b'; break;
			case 'f': decoded = '\f'; break;
			case 'n': decoded = '\n'; break;
			case 'r': decoded = '\r'; break;
			case 't': decoded = '\t'; break;
			case 'u':
				h0 = hex_digit(value[0]); h1 = hex_digit(value[1]);
				h2 = hex_digit(value[2]); h3 = hex_digit(value[3]);
				if (h0 < 0 || h1 < 0 || h2 < 0 || h3 < 0) return 0;
				codepoint = (h0 << 12) | (h1 << 8) | (h2 << 4) | h3;
				value += 4;
				if (codepoint > 127) return 0;
				decoded = (unsigned char)codepoint;
				break;
			default: break;
			}
		}
		out[used++] = (char)decoded;
	}
	out[used] = '\0';
	return 1;
}

static void clear_pressed_key(input_template_t *input) {
	input->keychar = 0;
	input->keycode = 0;
	input->special = 0;
}

static void set_key(input_template_t *input, const char *line) {
	char key[32];
	int down = strstr(line, "\"down\":true") != NULL;
	if (!parse_key(line, key, sizeof(key))) return;

	/* Modifiers are state, not characters. In particular, never turn the
	 * browser key name "Shift" into an S on the Atari keyboard. */
	if (!strcmp(key, "Shift")) { input->shift = down; return; }
	if (!strcmp(key, "Control")) { input->control = down; return; }
	if (!strcmp(key, "Alt") || !strcmp(key, "Meta")) {
		if (down) { clear_pressed_key(input); input->keycode = AKEY_ATARI; }
		else clear_pressed_key(input);
		return;
	}
	if (!strcmp(key, "F5")) { input->start = down; return; }
	if (!strcmp(key, "F6")) { input->select = down; return; }
	if (!strcmp(key, "F7")) { input->option = down; return; }

	if (!down) { clear_pressed_key(input); return; }
	clear_pressed_key(input);
	if (!strcmp(key, "Enter")) input->keychar = '\n';
	else if (!strcmp(key, "Backspace")) input->keychar = '\b';
	else if (!strcmp(key, "Tab")) input->keychar = '\t';
	else if (!strcmp(key, "Escape")) input->keychar = 27;
	else if (!strcmp(key, "Delete")) input->keychar = 127;
	else if (!strcmp(key, "Insert")) input->keycode = AKEY_INSERT_CHAR;
	else if (!strcmp(key, "CapsLock")) input->keycode = AKEY_CAPSTOGGLE;
	else if (!strcmp(key, "ArrowUp")) input->keycode = AKEY_UP;
	else if (!strcmp(key, "ArrowDown")) input->keycode = AKEY_DOWN;
	else if (!strcmp(key, "ArrowLeft")) input->keycode = AKEY_LEFT;
	else if (!strcmp(key, "ArrowRight")) input->keycode = AKEY_RIGHT;
	else if (!strcmp(key, "F1")) input->keycode = AKEY_F1;
	else if (!strcmp(key, "F2")) input->keycode = AKEY_F2;
	else if (!strcmp(key, "F3")) input->keycode = AKEY_F3;
	else if (!strcmp(key, "F4")) input->keycode = AKEY_F4;
	else if (key[0] != '\0' && key[1] == '\0' && (unsigned char)key[0] < 128)
		input->keychar = (unsigned char)key[0];
}

static void send_frame(int id, unsigned char *pixels) {
	unsigned char *screen = libatari800_get_screen_ptr();
	int x, y;
	unsigned char *out = pixels;
	for (y = 0; y < HEIGHT; y++) {
		for (x = 0; x < WIDTH; x++) {
			int rgb = Colours_table[screen[y * 384 + x + 24]];
			*out++ = (rgb >> 16) & 255;
			*out++ = (rgb >> 8) & 255;
			*out++ = rgb & 255;
		}
	}
	printf("{\"id\":%d,\"ok\":true,\"number\":%d,\"width\":%d,\"height\":%d,\"hostDevice\":%s,\"pixels\":\"",
		id, libatari800_get_frame_number(), WIDTH, HEIGHT,
		host_device_present() ? "true" : "false");
	write_base64(pixels, WIDTH * HEIGHT * 3);
	puts("\"}");
	fflush(stdout);
}

int main(int argc, char **argv) {
	const char *artifact = NULL, *eos = NULL, *host_dir = NULL, *machine = "xl", *memory = "64", *video = "ntsc";
	const char *drives[8] = { NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL };
	const char *rdevice_mode = "disabled", *rdevice_serial = NULL, *ide_image = NULL;
	int drive_readonly[8] = { 0, 0, 0, 0, 0, 0, 0, 0 };
	int basic = 0, sio_patch = 1, cpu_speed = 100, u1mb = 0;
	int rdevice_port = 9000, rdevice_baud = 2400, rdevice_modem = 0, ide_cf = 0;
	int frames_per_poll = 4;
	char *extra_args[64];
	char *emu_args[128];
	int emu_argc = 0, extra_argc = 0, i;
	char line[4096];
	input_template_t input;
	unsigned char *pixels;
	for (i = 1; i < argc; i++) {
		if (!strcmp(argv[i], "-artifact") && i + 1 < argc) artifact = argv[++i];
		else if (!strcmp(argv[i], "-eos") && i + 1 < argc) eos = argv[++i];
		else if (!strcmp(argv[i], "-host-dir") && i + 1 < argc) host_dir = argv[++i];
		else if (!strcmp(argv[i], "-machine") && i + 1 < argc) machine = argv[++i];
		else if (!strcmp(argv[i], "-memory") && i + 1 < argc) memory = argv[++i];
		else if (!strcmp(argv[i], "-video") && i + 1 < argc) video = argv[++i];
		else if (!strcmp(argv[i], "-cpu-speed") && i + 1 < argc) cpu_speed = atoi(argv[++i]);
		else if (!strcmp(argv[i], "-u1mb") && i + 1 < argc) u1mb = !strcmp(argv[++i], "true");
		else if (!strcmp(argv[i], "-basic") && i + 1 < argc) basic = !strcmp(argv[++i], "true");
		else if (!strcmp(argv[i], "-sio-patch") && i + 1 < argc) sio_patch = !strcmp(argv[++i], "true");
		else if (!strcmp(argv[i], "-drive1") && i + 1 < argc) drives[0] = argv[++i];
		else if (!strcmp(argv[i], "-drive2") && i + 1 < argc) drives[1] = argv[++i];
		else if (!strcmp(argv[i], "-drive3") && i + 1 < argc) drives[2] = argv[++i];
		else if (!strcmp(argv[i], "-drive4") && i + 1 < argc) drives[3] = argv[++i];
		else if (!strcmp(argv[i], "-drive8") && i + 1 < argc) drives[7] = argv[++i];
		else if (!strcmp(argv[i], "-drive1-readonly")) drive_readonly[0] = 1;
		else if (!strcmp(argv[i], "-drive2-readonly")) drive_readonly[1] = 1;
		else if (!strcmp(argv[i], "-drive3-readonly")) drive_readonly[2] = 1;
		else if (!strcmp(argv[i], "-drive4-readonly")) drive_readonly[3] = 1;
		else if (!strcmp(argv[i], "-rdevice-mode") && i + 1 < argc) rdevice_mode = argv[++i];
		else if (!strcmp(argv[i], "-rdevice-port") && i + 1 < argc) rdevice_port = atoi(argv[++i]);
		else if (!strcmp(argv[i], "-rdevice-baud") && i + 1 < argc) rdevice_baud = atoi(argv[++i]);
		else if (!strcmp(argv[i], "-rdevice-serial") && i + 1 < argc) rdevice_serial = argv[++i];
		else if (!strcmp(argv[i], "-rdevice-modem") && i + 1 < argc) rdevice_modem = !strcmp(argv[++i], "true");
		else if (!strcmp(argv[i], "-ide") && i + 1 < argc) ide_image = argv[++i];
		else if (!strcmp(argv[i], "-ide-cf")) ide_cf = 1;
		else if (!strcmp(argv[i], "-arg") && i + 1 < argc && extra_argc < 64) extra_args[extra_argc++] = argv[++i];
	}
	emu_args[emu_argc++] = "atari800";
	if (u1mb || !strcmp(memory, "1088")) emu_args[emu_argc++] = "-1088xe";
	else if (!strcmp(memory, "320-compy")) emu_args[emu_argc++] = "-320xe";
	else if (!strcmp(memory, "320-rambo")) emu_args[emu_argc++] = "-rambo";
	else if (!strcmp(memory, "256")) emu_args[emu_argc++] = "-256xe";
	else if (!strcmp(memory, "128")) emu_args[emu_argc++] = "-xe";
	else if (!strcmp(machine, "atari")) emu_args[emu_argc++] = "-atari";
	else if (!strcmp(machine, "xe")) emu_args[emu_argc++] = "-xe";
	else if (!strcmp(machine, "5200")) emu_args[emu_argc++] = "-5200";
	else emu_args[emu_argc++] = "-xl";
	emu_args[emu_argc++] = !strcmp(video, "pal") ? "-pal" : "-ntsc";
	emu_args[emu_argc++] = basic ? "-basic" : "-nobasic";
	if (!sio_patch) emu_args[emu_argc++] = "-nopatch";
	if (cpu_speed != 100) emu_args[emu_argc++] = "-turbo";
	emu_args[emu_argc++] = "-no-autosave-config";
	if (strcmp(rdevice_mode, "disabled")) {
		emu_args[emu_argc++] = "-rdevice";
		if (rdevice_serial) emu_args[emu_argc++] = (char *)rdevice_serial;
	}
	if (ide_image) {
		emu_args[emu_argc++] = "-ide";
		emu_args[emu_argc++] = (char *)ide_image;
		if (ide_cf) emu_args[emu_argc++] = "-ide_cf";
	}
	if (host_dir) {
		emu_args[emu_argc++] = "-H1";
		emu_args[emu_argc++] = (char *)host_dir;
		emu_args[emu_argc++] = "-hreadwrite";
	}
	for (i = 0; i < extra_argc; i++) emu_args[emu_argc++] = extra_args[i];
	if (eos && artifact) {
		emu_args[emu_argc++] = "-cart";
		emu_args[emu_argc++] = (char *)eos;
		emu_args[emu_argc++] = (char *)artifact;
	} else if (eos) {
		emu_args[emu_argc++] = "-cart";
		emu_args[emu_argc++] = (char *)eos;
	} else if (artifact && (strstr(artifact, ".atr") || strstr(artifact, ".ATR") || strstr(artifact, ".car") || strstr(artifact, ".CAR") || strstr(artifact, ".rom") || strstr(artifact, ".ROM"))) {
		emu_args[emu_argc++] = (char *)artifact;
	} else if (artifact) {
		emu_args[emu_argc++] = "-run";
		emu_args[emu_argc++] = (char *)artifact;
	}
	if (!libatari800_init(emu_argc, emu_args)) {
		fprintf(stderr, "Atari800 initialization failed: %s\n", libatari800_error_message());
		return 1;
	}
	if (host_dir) {
		/* Config files and advanced arguments must not be able to silently
		 * disable the project H1: mapping. Reapply it after all Atari800
		 * options have been parsed and rebuild the handler patch table. */
		strncpy(Devices_atari_h_dir[0], host_dir, FILENAME_MAX - 1);
		Devices_atari_h_dir[0][FILENAME_MAX - 1] = '\0';
		Devices_h_read_only = 0;
		Devices_enable_h_patch = 1;
		Devices_UpdatePatches();
	}
	Atari800_turbo_speed = cpu_speed;
	frames_per_poll = (4 * cpu_speed) / 100;
	if (frames_per_poll < 1) frames_per_poll = 1;
#ifdef R_IO_DEVICE
	if (strcmp(rdevice_mode, "disabled"))
		/* The selected line rate is part of both 850 and R-Verter profiles.
		 * R-Verter always exposes modem behaviour, including CONNECT replies. */
		RDevice_SetNetworkConfig(rdevice_port,
			(!strcmp(rdevice_mode, "rverter") || rdevice_modem) ? rdevice_baud : 0);
#else
	if (strcmp(rdevice_mode, "disabled")) {
		fprintf(stderr, "This Atari800 build does not include R: device support\n");
		libatari800_exit();
		return 1;
	}
#endif
	for (i = 0; i < 8; i++) {
		if (drives[i] && !SIO_Mount(i + 1, drives[i], drive_readonly[i])) {
			fprintf(stderr, "Unable to mount D%d: %s\n", i + 1, drives[i]);
			libatari800_exit();
			return 1;
		}
	}
	libatari800_continue_emulation_on_brk(1);
	libatari800_clear_input_array(&input);
	pixels = (unsigned char *)malloc(WIDTH * HEIGHT * 3);
	if (!pixels) return 1;
	while (fgets(line, sizeof(line), stdin)) {
		int id = request_id(line);
		if (strstr(line, "\"op\":\"frame\"")) {
			for (i = 0; i < frames_per_poll; i++) (void)libatari800_next_frame(&input);
			send_frame(id, pixels);
		} else if (strstr(line, "\"op\":\"key\"")) {
			set_key(&input, line);
			/* A browser can deliver keydown and keyup between two screen polls.
			 * Advance once on keydown so every press reaches the emulated POKEY. */
			if (strstr(line, "\"down\":true")) (void)libatari800_next_frame(&input);
			printf("{\"id\":%d,\"ok\":true,\"keychar\":%u,\"keycode\":%u,\"shift\":%u,\"control\":%u}\n",
				id, input.keychar, input.keycode, input.shift, input.control);
			fflush(stdout);
		} else if (strstr(line, "\"op\":\"releaseKeys\"")) {
			libatari800_clear_input_array(&input);
			printf("{\"id\":%d,\"ok\":true}\n", id);
			fflush(stdout);
		} else if (strstr(line, "\"op\":\"dismountD8\"")) {
			SIO_Dismount(8);
			printf("{\"id\":%d,\"ok\":true}\n", id);
			fflush(stdout);
		} else if (strstr(line, "\"op\":\"mountD8\"")) {
			if (!drives[7]) {
				printf("{\"id\":%d,\"ok\":false,\"error\":\"D8 is not configured\"}\n", id);
			} else if (!SIO_Mount(8, drives[7], drive_readonly[7])) {
				printf("{\"id\":%d,\"ok\":false,\"error\":\"unable to remount D8\"}\n", id);
			} else {
				printf("{\"id\":%d,\"ok\":true}\n", id);
			}
			fflush(stdout);
		} else if (strstr(line, "\"op\":\"stop\"")) {
			printf("{\"id\":%d,\"ok\":true}\n", id);
			fflush(stdout);
			break;
		} else {
			printf("{\"id\":%d,\"ok\":false,\"error\":\"unknown operation\"}\n", id);
			fflush(stdout);
		}
	}
	free(pixels);
	libatari800_exit();
	return 0;
}
