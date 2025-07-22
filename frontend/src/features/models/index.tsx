import { Header } from '@/components/layout/header'
import { Main } from '@/components/layout/main'
import { useModels } from './data/models'
import { ModelsTable } from './components/models-table'
import { modelsColumns } from './components/models-columns'
import { ModelsPrimaryButtons } from './components/models-primary-buttons'
import { ModelsDialogs } from './components/models-dialogs'
import { ModelsProvider } from './context/models-context'

export function ModelsPage() {
  const { data: models, isLoading, error } = useModels()

  if (error) {
    return (
      <div className='flex h-full items-center justify-center'>
        <div className='text-center'>
          <h2 className='text-lg font-semibold'>加载失败</h2>
          <p className='text-muted-foreground'>无法加载模型数据</p>
        </div>
      </div>
    )
  }

  return (
    <ModelsProvider>
      <Header
        title='模型管理'
        description='管理系统中的 AI 模型'
        actions={<ModelsPrimaryButtons />}
      />
      <Main>
        <ModelsTable
          columns={modelsColumns}
          data={models?.models?.edges?.map(edge => edge.node) || []}
          isLoading={isLoading}
        />
      </Main>
      <ModelsDialogs />
    </ModelsProvider>
  )
}