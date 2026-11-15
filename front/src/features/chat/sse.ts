export interface SseEvent {
  data: string;
  event: string;
}

export function createSseParser(onEvent: (event: SseEvent) => void) {
  let buffer = '';
  let event = '';
  let data: string[] = [];

  function dispatch() {
    if (data.length) onEvent({ event: event || 'message', data: data.join('\n') });
    event = '';
    data = [];
  }

  function line(text: string) {
    if (text === '') return dispatch();
    if (text.startsWith(':')) return;
    const colon = text.indexOf(':');
    const field = colon < 0 ? text : text.slice(0, colon);
    let value = colon < 0 ? '' : text.slice(colon + 1);
    if (value.startsWith(' ')) value = value.slice(1);
    if (field === 'event') event = value;
    else if (field === 'data') data.push(value);
  }

  return {
    feed(chunk: string) {
      buffer += chunk;
      const pattern = /\r\n|\r|\n/g;
      let start = 0;
      for (let match = pattern.exec(buffer); match; match = pattern.exec(buffer)) {
        if (match[0] === '\r' && match.index === buffer.length - 1) break;
        line(buffer.slice(start, match.index));
        start = match.index + match[0].length;
      }
      buffer = buffer.slice(start);
    },
    end() {
      if (buffer) line(buffer);
      buffer = '';
      dispatch();
    }
  };
}
