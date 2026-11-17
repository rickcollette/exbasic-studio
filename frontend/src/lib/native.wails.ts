// The production adapter: generated Wails bindings plus the Wails runtime.
import { Window, Dialogs, Clipboard, Events } from '@wailsio/runtime';
import * as StudioService from '../../bindings/exbasicstudio/internal/studio/studioservice';
import type { Native, NativeEvents, OpenDialogOptions, StudioAPI } from './native';

function asList(selected: unknown): string[] {
  if (Array.isArray(selected)) return selected.filter((item): item is string => typeof item === 'string' && item !== '');
  return typeof selected === 'string' && selected !== '' ? [selected] : [];
}

export function createWailsNative(): Native {
  return {
    studio: StudioService as unknown as StudioAPI,
    window: {
      minimise: () => void Window.Minimise(),
      toggleMaximise: () => void Window.ToggleMaximise(),
      close: () => void Window.Close(),
    },
    dialogs: {
      async open(options: OpenDialogOptions) {
        const selected = await Dialogs.OpenFile({
          Title: options.title,
          Directory: options.directory,
          CanChooseDirectories: options.chooseDirectories ?? false,
          CanChooseFiles: options.chooseFiles ?? true,
          AllowsMultipleSelection: options.multiple ?? false,
          Filters: options.filters?.map(filter => ({ DisplayName: filter.name, Pattern: filter.pattern })),
        });
        return asList(selected);
      },
      async save(options) {
        const selected = await Dialogs.SaveFile({
          Title: options.title,
          Filename: options.filename,
          Filters: options.filters?.map(filter => ({ DisplayName: filter.name, Pattern: filter.pattern })),
        });
        return typeof selected === 'string' ? selected : '';
      },
    },
    clipboard: { setText: async text => { await Clipboard.SetText(text); } },
    on<K extends keyof NativeEvents>(name: K, handler: (data: NativeEvents[K]) => void) {
      return Events.On(name, (event: { data?: unknown }) => {
        let data: any = event?.data ?? event;
        // Wails wraps a single emitted value in an array.
        if (Array.isArray(data) && data.length === 1) data = data[0];
        if (name === 'files-dropped') {
          data = { files: asList(data?.files), targetId: String(data?.details?.id ?? '') };
        }
        handler(data as NativeEvents[K]);
      });
    },
  };
}
