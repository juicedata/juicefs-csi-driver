import Editor from '@monaco-editor/react'
import { Modal } from 'antd'

const YamlModal: React.FC<{
  isOpen: boolean
  onClose: () => void
  content: string
}> = ({ isOpen, onClose, content }) => {
  return (
    <Modal title="YAML" open={isOpen} onCancel={onClose} footer={null}>
      <Editor
        defaultLanguage="yaml"
        options={{
          wordWrap: 'on',
          readOnly: true,
          theme: 'vs-light', // TODO dark mode
          scrollBeyondLastLine: false,
        }}
        value={content}
      />
    </Modal>
  )
}

export default YamlModal
