import { ReactNode, useEffect, useState } from 'react'
import Editor from '@monaco-editor/react'
import { Modal } from 'antd'

import { useWebsocket } from '@/hooks/use-api'

const LogModal: React.FC<{
  namespace: string
  name: string
  container: string
  children: ({ onClick }: { onClick: () => void }) => ReactNode
}> = ({ namespace, name, container, children }) => {
  const [isModalOpen, setIsModalOpen] = useState(false)

  const [socketUrl, setSocketUrl] = useState('')
  const { lastMessage } = useWebsocket(socketUrl, isModalOpen)

  const showModal = () => {
    setSocketUrl(`/api/v1/ws/pod/${namespace}/${name}/${container}/logs`)
    setIsModalOpen(true)
  }
  const handleOk = () => {
    setIsModalOpen(false)
  }
  const handleCancel = () => {
    setIsModalOpen(false)
  }

  const [data, setData] = useState<string>('')
  useEffect(() => {
    if (lastMessage) {
      setData((prev) => prev + lastMessage.data)
    }
  }, [lastMessage])

  useEffect(() => {
    if (!isModalOpen) {
      setSocketUrl('')
      setData('')
    }
  }, [isModalOpen])

  return (
    <>
      {children({ onClick: showModal })}
      <Modal
        title={`Logs: ${namespace}/${name}/${container}`}
        open={isModalOpen}
        onOk={handleOk}
        onCancel={handleCancel}
        footer={null}
        width="100vw"
        style={{
          maxWidth: '100vw',
          top: 0,
          paddingBottom: 0,
        }}
      >
        {isModalOpen && (
          <Editor
            height="100vh"
            defaultLanguage="yaml"
            options={{
              wordWrap: 'on',
              readOnly: true,
              theme: 'vs-light', // TODO dark mode
              scrollBeyondLastLine: false,
            }}
            value={data}
          />
        )}
      </Modal>
    </>
  )
}

export default LogModal
