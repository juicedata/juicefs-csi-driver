import Editor from '@monaco-editor/react'
import { Modal } from 'antd'

const YamlModal: React.FC<{
  isOpen: boolean
  onClose: () => void
  content: string
}> = ({ isOpen, onClose, content }) => {
  return (
    <Modal
      title="YAML"
      open={isOpen}
      onCancel={onClose}
      footer={null}
      width="100vw"
      style={{
        maxWidth: '100vw',
        top: 0,
        paddingBottom: 0,
      }}
    >
      <Editor
        height="100vh"
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
