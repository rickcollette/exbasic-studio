/*
 * exBASIC Studio headless bridge for ex800 1.0.0.
 *
 * This program links to libatari800 and is therefore distributed under the
 * GNU GPL version 2 or later, like ex800 itself. The Studio application
 * communicates with it as a separate process over newline-delimited JSON.
 *
 * Protocol version 2
 * ------------------
 * Every request is one JSON object on one line (at most REQUEST_MAX bytes)
 * with an integer "id" and a string "op"; every request gets exactly one
 * response line carrying the same id. Version 2 adds a handshake ("hello")
 * and the debugger: pause, continue, instruction/line stepping, run-until,
 * breakpoint replacement, CPU state, stack and bounded memory reads. The
 * version 1 operations (frame, key, releaseKeys, dismountD8, mountD8, stop)
 * are unchanged. The "joystick" operation (capability "joystick") sets
 * joystick 0 from Studio's keyboard joystick: "stick" is the Atari STICK()
 * value (15 neutral, a 0 bit per pressed direction) and "trigger" is true
 * while fire is held.
 *
 * The debugger uses the code-breakpoint support inherited from the upstream
 * Atari800 CPU core
 * (MONITOR_BREAK): while anything can stop the machine, the core calls
 * PLATFORM_Exit(TRUE) after every instruction, and this file provides that
 * function. When the machine stops, requests are served from inside that
 * call until one of them resumes execution.
 */
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include "config.h"
#include "libatari800/libatari800.h"
#include "colours.h"
#include "atari.h"
#include "cpu.h"
#include "devices.h"
#include "log.h"
#include "memory.h"
#include "monitor.h"
#include "sio.h"
#ifdef R_IO_DEVICE
#include "rdevice.h"
#endif

#define PROTOCOL_VERSION 2
#define WIDTH 336
#define HEIGHT 240
#define REQUEST_MAX 65536
#define MEMORY_READ_MAX 1024
#define ADDRESS_LIST_MAX 20000

/* Key codes that libatari800.h does not define (see akey.h). */
#define STUDIO_AKEY_CLEAR (0x40 | AKEY_LESS)
#define STUDIO_SPECIAL_BREAK 5

#define MARK_BREAKPOINT 1
#define MARK_LINE 2

#ifndef MONITOR_BREAK
#error "Build ex800 with --enable-monitorbreak: the debugger needs MONITOR_BREAK."
#endif

static const char base64_table[] =
	"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

enum run_mode { MODE_RUN, MODE_STEP_INSTRUCTION, MODE_STEP_LINE, MODE_STEP_OUT, MODE_UNTIL };

static input_template_t input;
static unsigned char *pixels;
static const char *drives[8];
static int drive_readonly[8];
static int frames_per_poll = 4;

/* Debugger state */
static unsigned char marks[0x10000];
static int breakpoint_count;
static int interp_dispatch = -1; /* address run once per bytecode instruction */
static int interp_pointer = -1;  /* zero page address of the bytecode pointer */
static enum run_mode mode = MODE_RUN;
static int pause_requested;
static int skip_checks;      /* instructions to execute before testing again */
static int until_address;
static int until_stack = -1; /* required stack pointer for MODE_UNTIL, or -1 */
static int call_depth;       /* JSR nesting seen by MODE_STEP_OUT */
static int stop_after_this;  /* MODE_STEP_OUT: the next instruction returns */
static int stopped;
static const char *stop_reason = "";
static int stop_frame_sent;  /* the screen of this stop was already sent */
static int pending_frame_id = -1;

/* ---- JSON helpers ----------------------------------------------------- */

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

/* Finds "name": in the request and returns the text after the colon. */
static const char *json_field(const char *line, const char *name) {
	char key[48];
	const char *value;
	snprintf(key, sizeof(key), "\"%s\":", name);
	value = strstr(line, key);
	if (!value) return NULL;
	value += strlen(key);
	while (*value == ' ') value++;
	return value;
}

static int json_int(const char *line, const char *name, long *out) {
	const char *value = json_field(line, name);
	char *end;
	if (!value) return 0;
	*out = strtol(value, &end, 10);
	return end != value;
}

static int json_true(const char *line, const char *name) {
	const char *value = json_field(line, name);
	return value && !strncmp(value, "true", 4);
}

static int json_op(const char *line, const char *op) {
	const char *value = json_field(line, "op");
	size_t len = strlen(op);
	return value && value[0] == '"' && !strncmp(value + 1, op, len) && value[1 + len] == '"';
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

static void reply_ok(int id) {
	printf("{\"id\":%d,\"ok\":true}\n", id);
	fflush(stdout);
}

static void reply_error(int id, const char *message) {
	printf("{\"id\":%d,\"ok\":false,\"error\":\"%s\"}\n", id, message);
	fflush(stdout);
}

/* ---- Keyboard --------------------------------------------------------- */

static void clear_pressed_key(input_template_t *in) {
	in->keychar = 0;
	in->keycode = 0;
	in->special = 0;
}

static void set_key(input_template_t *in, const char *line) {
	char key[32];
	int down = json_true(line, "down");
	if (!parse_key(line, key, sizeof(key))) return;

	/* Modifiers are state, not characters. In particular, never turn the
	 * browser key name "Shift" into an S on the Atari keyboard. */
	if (!strcmp(key, "Shift")) { in->shift = down; return; }
	if (!strcmp(key, "Control")) { in->control = down; return; }
	if (!strcmp(key, "Alt") || !strcmp(key, "Meta")) {
		if (down) { clear_pressed_key(in); in->keycode = AKEY_ATARI; }
		else clear_pressed_key(in);
		return;
	}
	if (!strcmp(key, "F5")) { in->start = down; return; }
	if (!strcmp(key, "F6")) { in->select = down; return; }
	if (!strcmp(key, "F7")) { in->option = down; return; }

	if (!down) { clear_pressed_key(in); return; }
	clear_pressed_key(in);
	if (!strcmp(key, "Enter")) in->keychar = '\n';
	else if (!strcmp(key, "Backspace")) in->keychar = '\b';
	else if (!strcmp(key, "Tab")) in->keychar = '\t';
	else if (!strcmp(key, "Escape")) in->keychar = 27;
	else if (!strcmp(key, "Delete")) in->keychar = 127;
	else if (!strcmp(key, "Insert")) in->keycode = AKEY_INSERT_CHAR;
	else if (!strcmp(key, "CapsLock")) in->keycode = AKEY_CAPSTOGGLE;
	else if (!strcmp(key, "ArrowUp")) in->keycode = AKEY_UP;
	else if (!strcmp(key, "ArrowDown")) in->keycode = AKEY_DOWN;
	else if (!strcmp(key, "ArrowLeft")) in->keycode = AKEY_LEFT;
	else if (!strcmp(key, "ArrowRight")) in->keycode = AKEY_RIGHT;
	else if (!strcmp(key, "F1")) in->keycode = AKEY_F1;
	else if (!strcmp(key, "F2")) in->keycode = AKEY_F2;
	else if (!strcmp(key, "F3")) in->keycode = AKEY_F3;
	else if (!strcmp(key, "F4")) in->keycode = AKEY_F4;
	/* Atari keys without a PC equivalent. CLEAR is Shift+<. BREAK has no
	 * key code: libatari800 reports -special, and akey.h's AKEY_BREAK is -5. */
	else if (!strcmp(key, "Home")) in->keycode = STUDIO_AKEY_CLEAR;
	else if (!strcmp(key, "End")) in->keycode = AKEY_HELP;
	else if (!strcmp(key, "Pause") || !strcmp(key, "F8")) in->special = STUDIO_SPECIAL_BREAK;
	else if (key[0] != '\0' && key[1] == '\0' && (unsigned char)key[0] < 128)
		in->keychar = (unsigned char)key[0];
}

/* ---- Joystick --------------------------------------------------------- */

/* Joystick 0 from a STICK() value and the trigger. libatari800 inverts joy0
 * when it builds PORTA, so joy0 holds a 1 bit per pressed direction. Returns
 * non-zero when the trigger has just been pressed. */
static int set_joystick(input_template_t *in, const char *line) {
	long stick = 15;
	int was_pressed = in->trig0 != 0;
	if (!json_int(line, "stick", &stick) || stick < 0 || stick > 15) stick = 15;
	in->joy0 = (UBYTE)(~stick & 0x0f);
	in->trig0 = (UBYTE)(json_true(line, "trigger") ? 1 : 0);
	return in->trig0 && !was_pressed;
}

/* ---- Machine state ---------------------------------------------------- */

static int host_device_present(void) {
	int address;
	for (address = 0x031a; address < 0x031a + 33; address += 3) {
		if (MEMORY_dGetByte(address) == 'H' && MEMORY_dGetWord(address + 1) == 0xd140)
			return 1;
	}
	return 0;
}

/* Reads memory without the side effects of hardware registers. */
static int peek(int address) {
	return MEMORY_SafeGetByte((UWORD)(address & 0xffff));
}

/* The code location of the CPU: the bytecode pointer while the interpreter
 * is about to run a bytecode instruction, the program counter otherwise. */
static int code_location(int *bytecode) {
	int pc = CPU_regPC;
	if (interp_dispatch >= 0 && pc == interp_dispatch) {
		*bytecode = 1;
		return peek(interp_pointer) | (peek(interp_pointer + 1) << 8);
	}
	*bytecode = 0;
	return pc;
}

static void print_debug_state(void) {
	int bytecode;
	int location = code_location(&bytecode);
	printf("\"debug\":{\"state\":\"%s\",\"reason\":\"%s\",\"pc\":%d,\"a\":%d,\"x\":%d,\"y\":%d,"
		"\"s\":%d,\"p\":%d,\"location\":%d,\"bytecode\":%s}",
		stopped ? "stopped" : "running", stopped ? stop_reason : "",
		CPU_regPC, CPU_regA, CPU_regX, CPU_regY, CPU_regS, CPU_regP,
		location, bytecode ? "true" : "false");
}

static void send_frame(int id, int with_pixels) {
	printf("{\"id\":%d,\"ok\":true,\"number\":%d,\"width\":%d,\"height\":%d,\"hostDevice\":%s,",
		id, libatari800_get_frame_number(), WIDTH, HEIGHT,
		host_device_present() ? "true" : "false");
	print_debug_state();
	fputs(",\"pixels\":\"", stdout);
	if (with_pixels) {
		unsigned char *screen = libatari800_get_screen_ptr();
		unsigned char *out = pixels;
		int x, y;
		for (y = 0; y < HEIGHT; y++) {
			for (x = 0; x < WIDTH; x++) {
				int rgb = Colours_table[screen[y * 384 + x + 24]];
				*out++ = (rgb >> 16) & 255;
				*out++ = (rgb >> 8) & 255;
				*out++ = rgb & 255;
			}
		}
		write_base64(pixels, WIDTH * HEIGHT * 3);
	}
	puts("\"}");
	fflush(stdout);
}

/* ---- Debugger --------------------------------------------------------- */

/* The CPU core calls the hook after every instruction only while something
 * can stop the machine, so normal execution keeps its full speed. */
static void arm(void) {
	MONITOR_break_step = (pause_requested || mode != MODE_RUN || breakpoint_count > 0) ? TRUE : FALSE;
}

/* Replaces or extends the addresses carrying one kind of mark. */
static int set_marks(const char *line, const char *field, int bit) {
	const char *value = json_field(line, field);
	const char *list_mode = json_field(line, "mode");
	int append = list_mode && !strncmp(list_mode, "\"append\"", 8);
	int count = 0, i;
	if (!value || *value != '[') return -1;
	if (!append) {
		for (i = 0; i < 0x10000; i++) marks[i] &= (unsigned char)~bit;
		if (bit == MARK_BREAKPOINT) breakpoint_count = 0;
	}
	value++;
	while (*value && *value != ']') {
		char *end;
		long address = strtol(value, &end, 10);
		if (end == value) return -1;
		if (address < 0 || address > 0xffff || ++count > ADDRESS_LIST_MAX) return -1;
		if (bit == MARK_BREAKPOINT && !(marks[address] & MARK_BREAKPOINT)) breakpoint_count++;
		marks[address] |= (unsigned char)bit;
		value = end;
		while (*value == ',' || *value == ' ') value++;
	}
	return *value == ']' ? count : -1;
}

static void resume(enum run_mode next) {
	mode = next;
	stopped = 0;
	stop_reason = "";
	stop_frame_sent = 0;
	/* Do not stop again on the instruction we are standing on. */
	skip_checks = 1;
	call_depth = 0;
	stop_after_this = 0;
	arm();
}

static int handle_request(const char *line);

static void enter_stop(const char *reason) {
	char *line;
	stopped = 1;
	stop_reason = reason;
	stop_frame_sent = 0;
	mode = MODE_RUN;
	pause_requested = 0;
	until_stack = -1;
	/* A key held when the machine stops would repeat forever. */
	libatari800_clear_input_array(&input);
	if (pending_frame_id >= 0) {
		/* Answer the frame request that was running when we stopped. */
		send_frame(pending_frame_id, 1);
		stop_frame_sent = 1;
		pending_frame_id = -1;
	}
	line = (char *)malloc(REQUEST_MAX);
	if (!line) exit(1);
	while (stopped) {
		if (!fgets(line, REQUEST_MAX, stdin)) {
			/* Studio closed the pipe: nothing can resume the machine. */
			free(line);
			exit(0);
		}
		if (handle_request(line)) {
			free(line);
			exit(0);
		}
	}
	free(line);
}

/* Called by the CPU core, through PLATFORM_Exit, after every instruction
 * while the debugger is armed. CPU_regPC is the next instruction. */
static void debug_hook(void) {
	int bytecode, location, opcode;
	if (pause_requested) { enter_stop("pause"); return; }
	/* The instruction that just ran was the return of a step-out. */
	if (stop_after_this) { enter_stop("step"); return; }
	if (skip_checks > 0) {
		skip_checks--;
	} else {
		location = code_location(&bytecode);
		if (marks[location] & MARK_BREAKPOINT) { enter_stop("breakpoint"); return; }
		switch (mode) {
		case MODE_STEP_INSTRUCTION:
			enter_stop("step");
			return;
		case MODE_STEP_LINE:
			if (marks[location] & MARK_LINE) { enter_stop("step"); return; }
			break;
		case MODE_UNTIL:
			if (location == until_address && (until_stack < 0 || CPU_regS >= until_stack)) {
				enter_stop("step");
				return;
			}
			break;
		default:
			break;
		}
	}
	if (mode == MODE_STEP_OUT) {
		/* Follow the 6502 return stack: the first return that is not
		 * matched by a call made since the step began leaves the routine. */
		opcode = peek(CPU_regPC);
		if (opcode == 0x20) call_depth++;
		else if (opcode == 0x60 || opcode == 0x40) {
			if (call_depth == 0) stop_after_this = 1;
			else call_depth--;
		}
	}
}

/* libatari800 leaves this function to the embedding program. A non-zero
 * result continues the emulation. */
int PLATFORM_Exit(int run_monitor) {
	Log_flushlog();
	if (!run_monitor) return 0;
	debug_hook();
	return 1;
}

/* ---- Requests --------------------------------------------------------- */

static void run_frames(int id) {
	int i;
	pending_frame_id = id;
	for (i = 0; i < frames_per_poll; i++) {
		(void)libatari800_next_frame(&input);
		/* A stop already answered this request; finish the current frame
		 * only, so execution resumes promptly after a continue. */
		if (pending_frame_id < 0) return;
	}
	pending_frame_id = -1;
	send_frame(id, 1);
}

/* Handles one request. Returns non-zero when the helper must exit. */
static int handle_request(const char *line) {
	long value = 0, length = 0;
	int id = json_int(line, "id", &value) ? (int)value : 0;

	if (!strchr(line, '\n') && strlen(line) >= REQUEST_MAX - 1) {
		/* Discard the rest of an oversized request. */
		int ch;
		while ((ch = getchar()) != EOF && ch != '\n') {}
		reply_error(id, "request too long");
		return 0;
	}
	if (json_op(line, "hello")) {
		printf("{\"id\":%d,\"ok\":true,\"protocol\":%d,\"helper\":\"atari800-studio\",\"core\":\"%s\","
			"\"capabilities\":[\"frame\",\"keyboard\",\"drives\",\"hostDevice\",\"debug\",\"memory\",\"joystick\"],"
			"\"limits\":{\"request\":%d,\"memory\":%d,\"addresses\":%d}}\n",
			id, PROTOCOL_VERSION, Atari800_TITLE, REQUEST_MAX, MEMORY_READ_MAX, ADDRESS_LIST_MAX);
		fflush(stdout);
	} else if (json_op(line, "frame")) {
		if (stopped) {
			/* The screen does not change while stopped: send it once. */
			send_frame(id, !stop_frame_sent);
			stop_frame_sent = 1;
		} else {
			run_frames(id);
		}
	} else if (json_op(line, "key")) {
		set_key(&input, line);
		/* A browser can deliver keydown and keyup between two screen polls.
		 * Advance once on keydown so every press reaches the emulated POKEY. */
		if (!stopped && json_true(line, "down")) (void)libatari800_next_frame(&input);
		printf("{\"id\":%d,\"ok\":true,\"keychar\":%u,\"keycode\":%u,\"shift\":%u,\"control\":%u}\n",
			id, input.keychar, input.keycode, input.shift, input.control);
		fflush(stdout);
	} else if (json_op(line, "joystick")) {
		/* As for keys, run one frame on a new press so a tap between two
		 * screen polls is still seen by the program. */
		if (set_joystick(&input, line) && !stopped) (void)libatari800_next_frame(&input);
		reply_ok(id);
	} else if (json_op(line, "releaseKeys")) {
		libatari800_clear_input_array(&input);
		reply_ok(id);
	} else if (json_op(line, "dismountD8")) {
		SIO_Dismount(8);
		reply_ok(id);
	} else if (json_op(line, "mountD8")) {
		if (!drives[7]) reply_error(id, "D8 is not configured");
		else if (!SIO_Mount(8, drives[7], drive_readonly[7])) reply_error(id, "unable to remount D8");
		else reply_ok(id);
	} else if (json_op(line, "state")) {
		printf("{\"id\":%d,\"ok\":true,", id);
		print_debug_state();
		puts("}");
		fflush(stdout);
	} else if (json_op(line, "setInterpreter")) {
		long dispatch = -1, pointer = -1;
		if (!json_int(line, "dispatch", &dispatch) || !json_int(line, "pointer", &pointer) ||
			dispatch < -1 || dispatch > 0xffff || pointer < -1 || pointer > 0xfe) {
			reply_error(id, "invalid interpreter description");
		} else {
			interp_dispatch = (int)dispatch;
			interp_pointer = dispatch < 0 ? -1 : (int)pointer;
			if (interp_pointer < 0) interp_dispatch = -1;
			reply_ok(id);
		}
	} else if (json_op(line, "setBreakpoints")) {
		if (set_marks(line, "addresses", MARK_BREAKPOINT) < 0) reply_error(id, "invalid breakpoint list");
		else { arm(); reply_ok(id); }
	} else if (json_op(line, "setLines")) {
		if (set_marks(line, "addresses", MARK_LINE) < 0) reply_error(id, "invalid line list");
		else reply_ok(id);
	} else if (json_op(line, "readMemory")) {
		long i;
		if (!json_int(line, "address", &value) || !json_int(line, "length", &length) ||
			value < 0 || value > 0xffff || length < 1 || length > MEMORY_READ_MAX || value + length > 0x10000) {
			reply_error(id, "invalid memory range");
		} else {
			printf("{\"id\":%d,\"ok\":true,\"address\":%ld,\"data\":\"", id, value);
			for (i = 0; i < length; i++) printf("%02x", peek((int)(value + i)));
			puts("\"}");
			fflush(stdout);
		}
	} else if (json_op(line, "pause")) {
		if (!stopped) { pause_requested = 1; arm(); }
		reply_ok(id);
	} else if (json_op(line, "continue") || json_op(line, "stepInstruction") || json_op(line, "stepLine") ||
			json_op(line, "stepOver") || json_op(line, "stepOut") || json_op(line, "runUntil")) {
		if (!stopped) {
			reply_error(id, "the machine is not stopped");
		} else if (json_op(line, "continue")) {
			resume(MODE_RUN);
			reply_ok(id);
		} else if (json_op(line, "stepInstruction")) {
			resume(MODE_STEP_INSTRUCTION);
			reply_ok(id);
		} else if (json_op(line, "stepLine")) {
			resume(MODE_STEP_LINE);
			reply_ok(id);
		} else if (json_op(line, "stepOut")) {
			int opcode = peek(CPU_regPC);
			resume(MODE_STEP_OUT);
			/* The instruction we stand on takes part in the call counting. */
			if (opcode == 0x20) call_depth++;
			else if (opcode == 0x60 || opcode == 0x40) stop_after_this = 1;
			reply_ok(id);
		} else if (json_op(line, "stepOver")) {
			if (peek(CPU_regPC) == 0x20) {
				/* Run the whole subroutine: stop at the instruction after
				 * the JSR once the stack is back where it is now. */
				until_address = (CPU_regPC + 3) & 0xffff;
				until_stack = CPU_regS;
				resume(MODE_UNTIL);
			} else {
				resume(MODE_STEP_INSTRUCTION);
			}
			reply_ok(id);
		} else {
			if (!json_int(line, "address", &value) || value < 0 || value > 0xffff) {
				reply_error(id, "invalid address");
			} else {
				until_address = (int)value;
				until_stack = -1;
				resume(MODE_UNTIL);
				reply_ok(id);
			}
		}
	} else if (json_op(line, "stop")) {
		reply_ok(id);
		return 1;
	} else {
		reply_error(id, "unknown operation");
	}
	return 0;
}

/* Case-insensitive file name suffix test. */
static int has_suffix(const char *name, const char *suffix) {
	size_t n = strlen(name), m = strlen(suffix), k;
	if (n < m) return 0;
	for (k = 0; k < m; k++) {
		char a = name[n - m + k], b = suffix[k];
		if (a >= 'A' && a <= 'Z') a = (char)(a - 'A' + 'a');
		if (a != b) return 0;
	}
	return 1;
}

int main(int argc, char **argv) {
	const char *artifact = NULL, *eos = NULL, *host_dir = NULL, *machine = "xl", *memory = "64", *video = "ntsc";
	const char *rdevice_mode = "disabled", *rdevice_serial = NULL, *ide_image = NULL;
	const char *cart = NULL, *cart_type = NULL, *cart2 = NULL, *cart2_type = NULL;
	int basic = 0, sio_patch = 1, cpu_speed = 100, u1mb = 0;
	int rdevice_port = 9000, rdevice_baud = 2400, rdevice_modem = 0, ide_cf = 0;
	char *extra_args[64];
	char *emu_args[128];
	int emu_argc = 0, extra_argc = 0, i;
	char *line;
	for (i = 1; i < argc; i++) {
		if (!strcmp(argv[i], "-artifact") && i + 1 < argc) artifact = argv[++i];
		else if (!strcmp(argv[i], "-eos") && i + 1 < argc) eos = argv[++i];
		else if (!strcmp(argv[i], "-cart") && i + 1 < argc) cart = argv[++i];
		else if (!strcmp(argv[i], "-cart-type") && i + 1 < argc) cart_type = argv[++i];
		else if (!strcmp(argv[i], "-cart2") && i + 1 < argc) cart2 = argv[++i];
		else if (!strcmp(argv[i], "-cart2-type") && i + 1 < argc) cart2_type = argv[++i];
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
	/* Cartridge slots. EOS in the primary slot is passed as -eos; any other
	 * primary cartridge as -cart, with its type when the image is raw. The
	 * secondary slot is Atari800's piggyback cartridge. */
	if (eos) {
		emu_args[emu_argc++] = "-cart";
		emu_args[emu_argc++] = (char *)eos;
	} else if (cart) {
		if (cart_type) {
			emu_args[emu_argc++] = "-cart-type";
			emu_args[emu_argc++] = (char *)cart_type;
		}
		emu_args[emu_argc++] = "-cart";
		emu_args[emu_argc++] = (char *)cart;
	}
	if (cart2) {
		if (cart2_type) {
			emu_args[emu_argc++] = "-cart2-type";
			emu_args[emu_argc++] = (char *)cart2_type;
		}
		emu_args[emu_argc++] = "-cart2";
		emu_args[emu_argc++] = (char *)cart2;
	}
	if (artifact && (eos || has_suffix(artifact, ".atr") || has_suffix(artifact, ".xfd") || has_suffix(artifact, ".car") || has_suffix(artifact, ".rom"))) {
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
	/* The monitor's own breakpoint must never match an executed address. */
	MONITOR_break_addr = 0xd000;
	MONITOR_break_step = FALSE;
	MONITOR_break_ret = FALSE;
	pixels = (unsigned char *)malloc(WIDTH * HEIGHT * 3);
	line = (char *)malloc(REQUEST_MAX);
	if (!pixels || !line) return 1;
	while (fgets(line, REQUEST_MAX, stdin)) {
		if (handle_request(line)) break;
	}
	free(line);
	free(pixels);
	libatari800_exit();
	return 0;
}
