export function FileTree({ files, readOnly, selected, disabled, onSelect }: {
  files: string[]; readOnly: string[]; selected: string; disabled?: boolean; onSelect: (path: string) => void
}) {
  const locks = new Set(readOnly)
  return <aside className="panel explorer" aria-label="Файлы проекта">
    <div className="panel-head"><h2>Файлы</h2><span className="counter">{files.length}</span></div>
    {files.length === 0 ? <p className="empty">Файлов пока нет.</p> :
      <ul>{files.map(path => {
        const boundary = path.lastIndexOf('/')
        const name = path.slice(boundary + 1)
        const parent = boundary < 0 ? '' : path.slice(0, boundary)
        return <li key={path}>
          <button className={`file-button ${path === selected ? 'selected' : ''}`}
            title={path} aria-current={path === selected ? 'true' : undefined}
            type="button" disabled={disabled} onClick={() => onSelect(path)}>
            <span className="file-icon" aria-hidden="true">{path.endsWith('.go') ? 'Go' : '≡'}</span>
            <span className="file-name">{name}{parent && <small>{parent}</small>}</span>
            {locks.has(path) && <span className="file-lock" role="img" aria-label="Только чтение" title="Только чтение">🔒</span>}
          </button>
        </li>
      })}</ul>}
  </aside>
}
