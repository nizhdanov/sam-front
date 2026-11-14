export function multipartConfig(body: { file: Blob }) {
  const data = new FormData();
  data.append('file', body.file);

  return { data };
}
