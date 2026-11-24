import { describe, expect, it } from 'vitest';
import { fileTypeLabel, hasDocumentPreview, imageMime, isImagePath, isJsonPath, languageFor } from './fileTypes';

describe('file types', () => {
  it('recognizes browser images and their MIME types', () => {
    expect(['a.png', 'a.JPG', 'a.webp', 'a.svg', 'a.avif'].every(isImagePath)).toBe(true);
    expect(isImagePath('a.pmg')).toBe(false);
    expect(imageMime('photo.jpg')).toBe('image/jpeg');
    expect(imageMime('icon.svg')).toBe('image/svg+xml');
  });

  it('provides language support for common project text formats', () => {
    for (const path of ['data.json', 'README.md', 'diagram.mmd', 'app.ts', 'index.html', 'style.css', 'config.yml', 'tool.py', 'main.cpp', 'lib.rs', 'query.sql']) {
      expect(languageFor(path), path).not.toBeNull();
    }
    expect(languageFor('notes.txt')).toBeNull();
    expect(isJsonPath('manifest.webmanifest')).toBe(true);
    expect(hasDocumentPreview('README.markdown')).toBe(true);
    expect(fileTypeLabel('flow.mermaid')).toBe('Mermaid diagram');
  });
});
