type ConfirmBarProps = {
  title: string;
  confirmLabel: string;
  onConfirm: () => void;
  onCancel: () => void;
  danger?: boolean;
};

export function ConfirmBar({ title, confirmLabel, onConfirm, onCancel, danger }: ConfirmBarProps) {
  return (
    <div className="confirm-bar" role="alertdialog" aria-label={title}>
      <p>{title}</p>
      <div className="confirm-bar__actions">
        <button type="button" className={danger ? "btn-danger" : undefined} onClick={onConfirm}>
          {confirmLabel}
        </button>
        <button type="button" className="btn-ghost" onClick={onCancel}>
          Cancel
        </button>
      </div>
    </div>
  );
}
